CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    action     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO audit_log (user_id, action, created_at) VALUES
    (1, 'user.created', now() - interval '2 days'),
    (1, 'user.login', now() - interval '1 day'),
    (1, 'profile.viewed', now() - interval '1 hour');
