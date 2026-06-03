CREATE TABLE trending_rankings (
    period          TEXT    NOT NULL,           -- 'daily'|'weekly'|'monthly'
    period_date     TEXT    NOT NULL,           -- YYYY-MM-DD (UTC)
    repository_id   INTEGER NOT NULL REFERENCES repositories(id),
    rank            INTEGER NOT NULL,
    score           REAL    NOT NULL,
    star_delta      INTEGER NOT NULL,
    language        TEXT,
    PRIMARY KEY (period, period_date, repository_id)
);
CREATE INDEX idx_rankings_lookup ON trending_rankings(period, period_date, rank);
CREATE INDEX idx_rankings_lang   ON trending_rankings(period, period_date, language, rank);
