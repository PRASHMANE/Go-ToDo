CREATE TABLE IF NOT EXISTS activities (
    id          UUID PRIMARY KEY,
    event_id    UUID         NOT NULL UNIQUE,   -- idempotency key
    user_id     UUID         NOT NULL,
    todo_id     UUID         NOT NULL,
    type        VARCHAR(50)  NOT NULL,
    title       VARCHAR(200) NOT NULL,
    occurred_at TIMESTAMPTZ  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activities_user_time ON activities (user_id, occurred_at DESC);
