package store

import (
	"database/sql"
	"errors"
)

// GetRepositoryByID 按内部 id 取仓库;不存在时 ok=false。
func (d *DB) GetRepositoryByID(id int64) (Repository, bool, error) {
	r, err := scanRepo(d.db.QueryRow(`SELECT `+repoSelectColumns+` FROM repositories WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, false, nil
	}
	if err != nil {
		return Repository{}, false, err
	}
	return r, true, nil
}

// BestDailyRank 返回该仓库在 daily 榜上拿到过的最佳(最小)名次;从未上榜时 ok=false。
func (d *DB) BestDailyRank(repoID int64) (int, bool, error) {
	var rank sql.NullInt64
	if err := d.db.QueryRow(
		`SELECT MIN(rank) FROM trending_rankings WHERE repository_id=? AND period='daily'`,
		repoID).Scan(&rank); err != nil {
		return 0, false, err
	}
	if !rank.Valid {
		return 0, false, nil
	}
	return int(rank.Int64), true, nil
}

// RepositorySnapshots 返回某仓库 [from, to] 内的快照,按日期升序;from/to 为空表示不设该端边界。
func (d *DB) RepositorySnapshots(repoID int64, from, to string) ([]Snapshot, error) {
	q := `SELECT repository_id, snapshot_date, stars, forks, open_issues, watchers, star_delta
FROM repository_snapshots WHERE repository_id=?`
	args := []any{repoID}
	if from != "" {
		q += ` AND snapshot_date >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND snapshot_date <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY snapshot_date`

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.RepositoryID, &s.Date, &s.Stars, &s.Forks, &s.OpenIssues, &s.Watchers, &s.StarDelta); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RankingHistory 是某仓库一条历史上榜记录。
type RankingHistory struct {
	Period    string
	Date      string
	Rank      int
	Score     float64
	StarDelta int
}

// RepositoryRankingHistory 返回某仓库所有周期的历史上榜记录,按日期降序、同日按 period。
func (d *DB) RepositoryRankingHistory(repoID int64) ([]RankingHistory, error) {
	rows, err := d.db.Query(`
SELECT period, period_date, rank, score, star_delta
FROM trending_rankings WHERE repository_id=?
ORDER BY period_date DESC, period`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RankingHistory
	for rows.Next() {
		var h RankingHistory
		if err := rows.Scan(&h.Period, &h.Date, &h.Rank, &h.Score, &h.StarDelta); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
