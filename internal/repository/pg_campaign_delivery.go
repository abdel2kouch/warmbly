package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warmbly/warmbly/internal/models"
)

// CampaignDeliveryContext is the campaign identity attached to a worker result.
// It is returned from the same transaction that finalizes the task so callers
// can emit realtime notifications without re-deriving task ownership.
type CampaignDeliveryContext struct {
	TaskID         uuid.UUID
	CampaignID     uuid.UUID
	ContactID      uuid.UUID
	SequenceID     uuid.UUID
	EmailAccountID uuid.UUID
	UserID         string
	OrganizationID *uuid.UUID
	CampaignName   string
	ContactEmail   string
	ContactName    string
	SequenceName   string
	SequenceIndex  int
}

type CampaignDeliveryRepository interface {
	FinalizeSuccess(context.Context, models.SendEmailResult) (*CampaignDeliveryContext, bool, error)
	FinalizeFailure(context.Context, models.SendEmailResult) (*CampaignDeliveryContext, bool, error)
}

type campaignDeliveryRepository struct{ db *pgxpool.Pool }

func NewCampaignDeliveryRepository(db *pgxpool.Pool) CampaignDeliveryRepository {
	return &campaignDeliveryRepository{db: db}
}

func (r *campaignDeliveryRepository) lockCampaignTask(ctx context.Context, tx pgx.Tx, taskID uuid.UUID) (*CampaignDeliveryContext, string, bool, error) {
	var d CampaignDeliveryContext
	var orgID *uuid.UUID
	var status string
	var isNewLead bool
	err := tx.QueryRow(ctx, `
		SELECT t.id, ct.campaign_id, ct.contact_id, ct.sequence_id,
		       t.email_account_id, t.status::text, c.user_id::text,
		       c.organization_id, c.name, co.email,
		       BTRIM(CONCAT_WS(' ', co.first_name, co.last_name)),
		       s.name, s.position + 1,
		       NOT EXISTS (
		         SELECT 1 FROM campaign_contact_progress p
		         WHERE p.campaign_id = ct.campaign_id
		           AND p.contact_id = ct.contact_id
		           AND p.sent_at IS NOT NULL
		       )
		FROM tasks t
		JOIN campaign_tasks ct ON ct.task_id = t.id
		JOIN campaigns c ON c.id = ct.campaign_id
		JOIN contacts co ON co.id = ct.contact_id
		JOIN sequences s ON s.id = ct.sequence_id
		WHERE t.id = $1 AND t.task_type = 'campaign'
		FOR UPDATE OF t
	`, taskID).Scan(
		&d.TaskID, &d.CampaignID, &d.ContactID, &d.SequenceID,
		&d.EmailAccountID, &status, &d.UserID, &orgID, &d.CampaignName,
		&d.ContactEmail, &d.ContactName, &d.SequenceName, &d.SequenceIndex,
		&isNewLead,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", false, nil
	}
	d.OrganizationID = orgID
	return &d, status, isNewLead, err
}

func (r *campaignDeliveryRepository) FinalizeSuccess(ctx context.Context, result models.SendEmailResult) (*CampaignDeliveryContext, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	d, status, isNewLead, err := r.lockCampaignTask(ctx, tx, result.TaskID)
	if err != nil || d == nil || status != "active" {
		return d, false, err
	}
	sentAt := result.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now().UTC()
	}
	providerID := result.ProviderMsgID
	messageID := result.MessageID
	if messageID == "" {
		messageID = providerID
	}

	if _, err = tx.Exec(ctx, `
		UPDATE tasks
		SET status = 'completed', message_id = COALESCE(NULLIF($2, ''), message_id),
		    completed_at = $3, updated_at = NOW()
		WHERE id = $1
	`, result.TaskID, messageID, sentAt); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO campaign_contact_progress (campaign_id, contact_id, sequence_id, sent_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (campaign_id, contact_id, sequence_id)
		DO UPDATE SET sent_at = EXCLUDED.sent_at
	`, d.CampaignID, d.ContactID, d.SequenceID, sentAt); err != nil {
		return nil, false, err
	}
	newLeadInc := 0
	if isNewLead {
		newLeadInc = 1
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO campaign_daily_sends (campaign_id, send_date, emails_sent, new_leads_started)
		VALUES ($1, ($2 AT TIME ZONE 'UTC')::date, 1, $3)
		ON CONFLICT (campaign_id, send_date) DO UPDATE
		SET emails_sent = campaign_daily_sends.emails_sent + 1,
		    new_leads_started = campaign_daily_sends.new_leads_started + EXCLUDED.new_leads_started
	`, d.CampaignID, sentAt, newLeadInc); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE campaign_senders
		SET rotation_position = rotation_position + 1, last_sent_at = $3
		WHERE campaign_id = $1 AND email_account_id = $2
	`, d.CampaignID, d.EmailAccountID, sentAt); err != nil {
		return nil, false, err
	}
	metadata, _ := json.Marshal(map[string]string{
		"task_id": result.TaskID.String(), "contact_id": d.ContactID.String(),
		"sequence_id": d.SequenceID.String(), "account_id": d.EmailAccountID.String(),
		"provider_message_id": providerID,
	})
	if _, err = tx.Exec(ctx, `
		INSERT INTO campaign_logs (campaign_id, event_type, message, metadata, created_at)
		VALUES ($1, 'email_sent', $2, $3::jsonb, $4)
	`, d.CampaignID, "Email sent to "+d.ContactEmail, metadata, sentAt); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM task_failures WHERE task_id = $1`, result.TaskID); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return d, true, nil
}

func (r *campaignDeliveryRepository) FinalizeFailure(ctx context.Context, result models.SendEmailResult) (*CampaignDeliveryContext, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	d, status, _, err := r.lockCampaignTask(ctx, tx, result.TaskID)
	if err != nil || d == nil || status != "active" {
		return d, false, err
	}
	code, message := "SEND_FAILED", result.LegacyErrorMsg
	if result.Error != nil {
		if result.Error.Code != "" {
			code = result.Error.Code
		}
		if result.Error.Message != "" {
			message = result.Error.Message
		}
	}
	if message == "" {
		message = "The email provider did not accept the message"
	}
	if _, err = tx.Exec(ctx, `
		UPDATE tasks SET status = 'failed', updated_at = NOW() WHERE id = $1
	`, result.TaskID); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO task_failures (task_id, title, message) VALUES ($1, $2, $3)
		ON CONFLICT (task_id) DO UPDATE SET title = EXCLUDED.title, message = EXCLUDED.message
	`, result.TaskID, code, message); err != nil {
		return nil, false, err
	}
	metadata, _ := json.Marshal(map[string]string{
		"level": "error", "task_id": result.TaskID.String(), "code": code,
		"contact_id": d.ContactID.String(), "sequence_id": d.SequenceID.String(),
		"account_id": d.EmailAccountID.String(), "error": message,
	})
	if _, err = tx.Exec(ctx, `
		INSERT INTO campaign_logs (campaign_id, event_type, message, metadata)
		VALUES ($1, 'email_failed', $2, $3::jsonb)
	`, d.CampaignID, "Email failed for "+d.ContactEmail+": "+message, metadata); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return d, true, nil
}
