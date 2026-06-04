CREATE TABLE users (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    provider         TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    login            TEXT NOT NULL,
    email            TEXT,
    avatar_url       TEXT,
    created_at       TEXT NOT NULL,
    UNIQUE(provider, provider_user_id)
);

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

ALTER TABLE submissions ADD COLUMN user_id INTEGER REFERENCES users(id);
