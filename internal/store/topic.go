package store

// TopicCount 是某话题及其下的活跃仓库数。
type TopicCount struct {
	Slug  string
	Name  string
	Count int
}

// SetRepositoryTopics 在事务里把某仓库的话题关联重置为 topics:
// upsert 每个 topic(slug=name=原串),清除旧关联后重新插入。
func (d *DB) SetRepositoryTopics(repoID int64, topics []string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM repository_topics WHERE repository_id=?`, repoID); err != nil {
		tx.Rollback()
		return err
	}
	for _, topic := range topics {
		if topic == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO topics(slug, name) VALUES(?, ?)`, topic, topic); err != nil {
			tx.Rollback()
			return err
		}
		var topicID int64
		if err := tx.QueryRow(`SELECT id FROM topics WHERE slug=?`, topic).Scan(&topicID); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO repository_topics(repository_id, topic_id) VALUES(?, ?)`,
			repoID, topicID); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetRepositoryTopics 返回某仓库的话题 slug(按 slug 升序)。
func (d *DB) GetRepositoryTopics(repoID int64) ([]string, error) {
	rows, err := d.db.Query(`
SELECT t.slug FROM repository_topics rt JOIN topics t ON t.id = rt.topic_id
WHERE rt.repository_id=? ORDER BY t.slug`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListTopics 返回有「达标活跃仓库」关联的话题及其计数,按数量降序、slug 升序。
// 计数口径与 Stats / 话题详情一致(见 minStatsStars):只计入 is_active=1 且 stars>=门槛 的仓库,
// 故仅由未达标仓库支撑的话题不会出现在列表里。
func (d *DB) ListTopics() ([]TopicCount, error) {
	rows, err := d.db.Query(`
SELECT t.slug, t.name, COUNT(rt.repository_id)
FROM topics t
JOIN repository_topics rt ON rt.topic_id = t.id
JOIN repositories r ON r.id = rt.repository_id
WHERE r.is_active=1 AND r.stars >= ?
GROUP BY t.id
ORDER BY COUNT(rt.repository_id) DESC, t.slug`, minStatsStars)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopicCount
	for rows.Next() {
		var tc TopicCount
		if err := rows.Scan(&tc.Slug, &tc.Name, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// CountRepositoriesByTopic 返回某话题下「达标活跃仓库」总数(口径见 minStatsStars)。
func (d *DB) CountRepositoriesByTopic(slug string) (int, error) {
	var n int
	err := d.db.QueryRow(`
SELECT COUNT(*) FROM repository_topics rt
JOIN topics t ON t.id = rt.topic_id
JOIN repositories r ON r.id = rt.repository_id
WHERE t.slug=? AND r.is_active=1 AND r.stars >= ?`, slug, minStatsStars).Scan(&n)
	return n, err
}

// RepositoriesByTopic 返回某话题下「达标活跃仓库」,按 stars 降序、full_name 分页(口径见 minStatsStars)。
func (d *DB) RepositoriesByTopic(slug string, limit, offset int) ([]Repository, error) {
	rows, err := d.db.Query(`SELECT `+repoColsR+`
FROM repository_topics rt
JOIN topics t ON t.id = rt.topic_id
JOIN repositories r ON r.id = rt.repository_id
WHERE t.slug=? AND r.is_active=1 AND r.stars >= ?
ORDER BY r.stars DESC, r.full_name LIMIT ? OFFSET ?`, slug, minStatsStars, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		r, err := scanRepoRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
