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
