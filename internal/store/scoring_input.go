package store

// DeltaRow 是某仓库某天的 star 增量(评分输入)。
type DeltaRow struct {
	RepositoryID int64
	Date         string
	StarDelta    int
}

// DailyDeltasInRange 返回 [from, to] 内所有快照的 star 增量,按 (repository_id, snapshot_date) 升序。
func (d *DB) DailyDeltasInRange(from, to string) ([]DeltaRow, error) {
	rows, err := d.db.Query(`
SELECT repository_id, snapshot_date, star_delta
FROM repository_snapshots
WHERE snapshot_date >= ? AND snapshot_date <= ?
ORDER BY repository_id, snapshot_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeltaRow
	for rows.Next() {
		var r DeltaRow
		if err := rows.Scan(&r.RepositoryID, &r.Date, &r.StarDelta); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RepoMeta 是评分需要的仓库元信息。
type RepoMeta struct {
	ID       int64
	Language string
	Stars    int
}

// ActiveRepoMeta 返回所有活跃仓库的 id->元信息(语言 + 当前 star 数)。
func (d *DB) ActiveRepoMeta() (map[int64]RepoMeta, error) {
	rows, err := d.db.Query(`
SELECT id, COALESCE(language,''), stars
FROM repositories WHERE is_active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]RepoMeta)
	for rows.Next() {
		var m RepoMeta
		if err := rows.Scan(&m.ID, &m.Language, &m.Stars); err != nil {
			return nil, err
		}
		out[m.ID] = m
	}
	return out, rows.Err()
}
