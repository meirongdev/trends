CREATE TABLE topics (
    id   INTEGER PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE repository_topics (
    repository_id INTEGER NOT NULL REFERENCES repositories(id),
    topic_id      INTEGER NOT NULL REFERENCES topics(id),
    PRIMARY KEY (repository_id, topic_id)
);
CREATE INDEX idx_repo_topics_topic ON repository_topics(topic_id);
