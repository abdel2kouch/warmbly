-- Repair the one production campaign affected before worker-confirmed delivery
-- accounting was introduced. The worker logs are the delivery authority: these
-- 23 task IDs have Gmail provider message IDs; the campaign's other historical
-- "completed" tasks were queue acknowledgements whose Gmail calls returned 429.
CREATE TEMP TABLE incident_confirmed_campaign_tasks (
    task_id uuid PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO incident_confirmed_campaign_tasks (task_id) VALUES
    ('664cbeb7-94f0-4bff-a8a3-69fa8f447c42'),
    ('14816d2d-35a9-4f30-b3e7-7258c65cdb1c'),
    ('70656a3d-5d50-4038-a7a9-6975e22f6abd'),
    ('78c73573-65b4-455a-b6f4-cc754ae7cc40'),
    ('0bcd24ce-3c01-4162-a972-e0c714a1f521'),
    ('7f787362-6f9d-4c24-91e2-cf8607353ee0'),
    ('bec18374-c7fe-4388-b13e-9ab8f0176a34'),
    ('96281690-c4e7-40ce-bd92-51c936ab4705'),
    ('64f9ca20-fcb3-4173-aaa8-cd38e5e714ba'),
    ('cdb2ae39-2b3f-4e98-a36f-db1879d7f336'),
    ('28bd197c-f934-4451-85ec-8749e4c1c4ab'),
    ('1cadfd1f-e5b2-4102-869c-8c7c37cf0d15'),
    ('d161d280-68d4-4f13-8d70-3cbbc097dca3'),
    ('941593d0-4651-438c-ac67-2c5cd5f8677d'),
    ('b4d9a989-3e82-46cd-a5ec-f4701bb1e01b'),
    ('d352d54d-83b4-477c-b1c7-30658986615f'),
    ('1fc050c3-214f-4e96-a1ec-e6da833a3747'),
    ('dbcf2ab4-b256-4ef4-9166-f4b6849d985d'),
    ('004f6401-cc21-4c20-98d2-7b4394641584'),
    ('26168a0d-8f02-4d14-be1b-277721fcfcaf'),
    ('5090d84e-eaa9-46ba-bf04-53b9af2bac47'),
    ('93add07d-211b-4272-965e-15dd54cb6d01'),
    ('a5bdc02f-7246-48fc-91dc-3b1c7fe68e38');

-- Remove false progress before changing task status so every affected contact
-- becomes eligible again when the owner resumes the campaign.
DELETE FROM campaign_contact_progress p
USING campaign_tasks ct, tasks t
WHERE ct.campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31'
  AND ct.task_id = t.id
  AND p.campaign_id = ct.campaign_id
  AND p.contact_id = ct.contact_id
  AND p.sequence_id = ct.sequence_id
  AND NOT EXISTS (
      SELECT 1 FROM incident_confirmed_campaign_tasks ok WHERE ok.task_id = t.id
  );

UPDATE tasks t
SET status = 'failed', completed_at = NULL, updated_at = NOW()
FROM campaign_tasks ct
WHERE ct.task_id = t.id
  AND ct.campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31'
  AND t.status = 'completed'
  AND NOT EXISTS (
      SELECT 1 FROM incident_confirmed_campaign_tasks ok WHERE ok.task_id = t.id
  );

INSERT INTO task_failures (task_id, title, message)
SELECT t.id, 'RATE_LIMIT_EXCEEDED',
       'Gmail rejected this delivery with a temporary user-rate limit; it was not sent.'
FROM tasks t
JOIN campaign_tasks ct ON ct.task_id = t.id
WHERE ct.campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31'
  AND t.status = 'failed'
  AND NOT EXISTS (
      SELECT 1 FROM incident_confirmed_campaign_tasks ok WHERE ok.task_id = t.id
  )
ON CONFLICT (task_id) DO UPDATE
SET title = EXCLUDED.title, message = EXCLUDED.message;

DELETE FROM task_failures tf
USING incident_confirmed_campaign_tasks ok
WHERE tf.task_id = ok.task_id;

-- Rebuild activity from confirmed task rows so the recent log agrees with the
-- analytics counters and lead progress.
DELETE FROM campaign_logs
WHERE campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31'
  AND event_type = 'email_sent';

INSERT INTO campaign_logs (campaign_id, event_type, message, metadata, created_at)
SELECT ct.campaign_id, 'email_sent', 'Email sent to ' || co.email,
       jsonb_build_object(
           'task_id', t.id::text,
           'contact_id', ct.contact_id::text,
           'sequence_id', ct.sequence_id::text,
           'account_id', t.email_account_id::text,
           'reconciled', true
       ),
       COALESCE(t.completed_at, t.updated_at)
FROM incident_confirmed_campaign_tasks ok
JOIN tasks t ON t.id = ok.task_id
JOIN campaign_tasks ct ON ct.task_id = t.id
JOIN contacts co ON co.id = ct.contact_id
WHERE ct.campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31';

-- All confirmed rows are first-step sends in this one-step campaign.
INSERT INTO campaign_daily_sends (campaign_id, send_date, emails_sent, new_leads_started)
SELECT 'a3e27b70-0065-4444-afd3-5545a5cb0c31',
       (MIN(COALESCE(t.completed_at, t.updated_at)) AT TIME ZONE 'UTC')::date,
       COUNT(*)::integer, COUNT(*)::integer
FROM incident_confirmed_campaign_tasks ok
JOIN tasks t ON t.id = ok.task_id
HAVING COUNT(*) > 0
ON CONFLICT (campaign_id, send_date) DO UPDATE
SET emails_sent = EXCLUDED.emails_sent,
    new_leads_started = EXCLUDED.new_leads_started;

UPDATE campaign_senders cs
SET rotation_position = confirmed.sent_count,
    last_sent_at = confirmed.last_sent_at
FROM (
    SELECT t.email_account_id, COUNT(*)::integer AS sent_count,
           MAX(COALESCE(t.completed_at, t.updated_at)) AS last_sent_at
    FROM incident_confirmed_campaign_tasks ok
    JOIN tasks t ON t.id = ok.task_id
    GROUP BY t.email_account_id
) confirmed
WHERE cs.campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31'
  AND cs.email_account_id = confirmed.email_account_id;

UPDATE campaign_senders
SET rotation_position = 0, last_sent_at = NULL
WHERE campaign_id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31'
  AND email_account_id NOT IN (
      SELECT t.email_account_id
      FROM incident_confirmed_campaign_tasks ok JOIN tasks t ON t.id = ok.task_id
  );

INSERT INTO campaign_logs (campaign_id, event_type, message, metadata)
SELECT c.id,
       'delivery_reconciled',
       'Corrected historical reporting to 23 Gmail-confirmed deliveries; rejected attempts were returned to the pending lead pool.',
       '{"confirmed":23,"source":"worker_provider_results"}'::jsonb
FROM campaigns c
WHERE c.id = 'a3e27b70-0065-4444-afd3-5545a5cb0c31';
