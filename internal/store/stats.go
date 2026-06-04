package store

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
	if err := d.db.QueryRow(`SELECT COUNT(DISTINCT language) FROM repositories WHERE is_active=1 AND language IS NOT NULL AND language <> ''`).Scan(&s.Languages); err != nil {
		return s, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(DISTINCT topic_id) FROM repository_topics`).Scan(&s.Topics); err != nil {
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
