package store

import (
	"database/sql"
	"errors"
)

type Snapshot struct {
	RepositoryID int64
	Date         string // YYYY-MM-DD (UTC)
	Stars        int
	Forks        int
	OpenIssues   int
	Watchers     int
	StarDelta    int
}

// InsertSnapshot 写入(或覆盖)某仓库某天的快照,对同日重跑幂等。
func (d *DB) InsertSnapshot(s Snapshot) error {
	_, err := d.db.Exec(`
INSERT INTO repository_snapshots
  (repository_id, snapshot_date, stars, forks, open_issues, watchers, star_delta)
VALUES (?,?,?,?,?,?,?)
ON CONFLICT(repository_id, snapshot_date) DO UPDATE SET
  stars=excluded.stars, forks=excluded.forks, open_issues=excluded.open_issues,
  watchers=excluded.watchers, star_delta=excluded.star_delta`,
		s.RepositoryID, s.Date, s.Stars, s.Forks, s.OpenIssues, s.Watchers, s.StarDelta)
	return err
}

// LastStars 返回该仓库最近一天快照的 star 数;无快照时 ok=false。
func (d *DB) LastStars(repositoryID int64) (int, bool, error) {
	var stars int
	err := d.db.QueryRow(`
SELECT stars FROM repository_snapshots
WHERE repository_id=?
ORDER BY snapshot_date DESC LIMIT 1`, repositoryID).Scan(&stars)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return stars, true, nil
}

// StarsBefore 返回该仓库在 date 之前(不含当天)最近一次快照的 star 数;
// 没有更早的快照时 ok=false。Snapshot 作业用它作增量基线,排除当天以保证同日重跑幂等。
func (d *DB) StarsBefore(repositoryID int64, date string) (int, bool, error) {
	var stars int
	err := d.db.QueryRow(`
SELECT stars FROM repository_snapshots
WHERE repository_id=? AND snapshot_date < ?
ORDER BY snapshot_date DESC LIMIT 1`, repositoryID, date).Scan(&stars)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return stars, true, nil
}
