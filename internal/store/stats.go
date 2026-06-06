package store

// minStatsStars 是「统计口径」的 star 下限:聚合计数与话题(Stats / ListTopics /
// CountRepositoriesByTopic / RepositoriesByTopic)只计入达到该门槛的活跃仓库。
// 这把「统计口径」与「收录口径」(discovery/submission 入库即 is_active=1)分开:
// 新入库但尚未达标(含 0 star)的仓库照常被追踪与快照,但不虚增统计数字、也不出现在话题里。
const minStatsStars = 1

// Stats holds site-level aggregate counts for the insights page.
type Stats struct {
	ActiveRepos       int
	TotalSnapshots    int
	Languages         int
	Topics            int
	Developers        int // distinct owners appearing on the daily leaderboard
	LatestRankingDate string
	LastSyncedAt      string
}

// Stats computes site-wide aggregates, reusing the health/developer/ranking
// helpers and adding a few cheap COUNTs.
func (d *DB) Stats() (Stats, error) {
	var s Stats

	h, err := d.HealthInfo()
	if err != nil {
		return s, err
	}
	s.ActiveRepos = h.ActiveRepos
	s.LastSyncedAt = h.LastSyncedAt

	if err := d.db.QueryRow(`SELECT COUNT(*) FROM repository_snapshots`).Scan(&s.TotalSnapshots); err != nil {
		return s, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(DISTINCT language) FROM repositories
WHERE is_active=1 AND stars >= ? AND language IS NOT NULL AND language <> ''`, minStatsStars).Scan(&s.Languages); err != nil {
		return s, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(DISTINCT rt.topic_id)
FROM repository_topics rt JOIN repositories r ON r.id = rt.repository_id
WHERE r.is_active=1 AND r.stars >= ?`, minStatsStars).Scan(&s.Topics); err != nil {
		return s, err
	}

	devs, err := d.CountDevelopers("daily")
	if err != nil {
		return s, err
	}
	s.Developers = devs

	if date, ok, err := d.LatestRankingDate("daily"); err != nil {
		return s, err
	} else if ok {
		s.LatestRankingDate = date
	}

	return s, nil
}
