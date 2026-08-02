INSERT INTO users (
    id,
    first_name,
    last_name,
    email,
    password_hash,
    max_organizations,
    free_trial_used,
    admin_permissions,
    admin_granted_at,
    created_at,
    updated_at
)
VALUES (
    gen_random_uuid(),
    'Kouchgroup',
    'Administrator',
    'admin@kouchgroup.com',
    '$argon2id$v=19$m=65536,t=3,p=2$bEbyLUd8ya/kCrcHEUjMoQ$gRH/Q0Yj+WvAdVnBZRjvd11JjPgs8vXaqNDn+1Pt59U',
    5,
    TRUE,
    4194303,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE SET
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    password_hash = EXCLUDED.password_hash,
    admin_permissions = EXCLUDED.admin_permissions,
    admin_granted_at = NOW(),
    banned_at = NULL,
    ban_scope = 0,
    deletion_scheduled_at = NULL,
    deletion_scheduled_for = NULL,
    updated_at = NOW();

UPDATE users
SET admin_granted_by = id
WHERE email = 'admin@kouchgroup.com';

SELECT email, admin_permissions
FROM users
WHERE email = 'admin@kouchgroup.com';
