CREATE TABLE repositories (
    id              INTEGER PRIMARY KEY,
    github_id       INTEGER NOT NULL UNIQUE,
    node_id         TEXT    NOT NULL,
    full_name       TEXT    NOT NULL UNIQUE,
    owner           TEXT    NOT NULL,
    name            TEXT    NOT NULL,
    description     TEXT,
    language        TEXT,
    homepage        TEXT,
    html_url        TEXT    NOT NULL,
    owner_avatar    TEXT,
    stars           INTEGER NOT NULL DEFAULT 0,
    forks           INTEGER NOT NULL DEFAULT 0,
    open_issues     INTEGER NOT NULL DEFAULT 0,
    watchers        INTEGER NOT NULL DEFAULT 0,
    is_archived     INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 1,
    repo_created_at TEXT,
    first_seen_at   TEXT    NOT NULL,
    last_synced_at  TEXT
);
CREATE INDEX idx_repos_language ON repositories(language);
CREATE INDEX idx_repos_active   ON repositories(is_active);
CREATE INDEX idx_repos_stars    ON repositories(stars DESC);

CREATE TABLE repository_snapshots (
    repository_id   INTEGER NOT NULL REFERENCES repositories(id),
    snapshot_date   TEXT    NOT NULL,
    stars           INTEGER NOT NULL,
    forks           INTEGER NOT NULL,
    open_issues     INTEGER NOT NULL,
    watchers        INTEGER NOT NULL DEFAULT 0,
    star_delta      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (repository_id, snapshot_date)
);
CREATE INDEX idx_snapshots_date ON repository_snapshots(snapshot_date);
