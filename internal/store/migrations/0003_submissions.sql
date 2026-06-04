CREATE TABLE submissions (
    id           INTEGER PRIMARY KEY,
    full_name    TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'pending',  -- pending|accepted|rejected
    submitted_ip TEXT,
    note         TEXT,
    created_at   TEXT    NOT NULL
);
CREATE INDEX idx_submissions_status ON submissions(status);
