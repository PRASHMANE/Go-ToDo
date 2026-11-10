CREATE TABLE IF NOT EXISTS todos (
    id          UUID PRIMARY KEY,
    user_id     UUID         NOT NULL,
    title       VARCHAR(200) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    completed   BOOLEAN      NOT NULL DEFAULT FALSE,
    priority    VARCHAR(10)  NOT NULL DEFAULT 'medium'
                CHECK (priority IN ('low', 'medium', 'high')),
    due_date    TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Almost every query is "this user's todos, newest first".
CREATE INDEX IF NOT EXISTS idx_todos_user_created ON todos (user_id, created_at DESC);
