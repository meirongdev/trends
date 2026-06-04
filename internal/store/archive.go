package store

// ArchiveEntry 是某仓库在某 period 榜单上的历史聚合(曾上榜的全部记录汇总)。
type ArchiveEntry struct {
	Repo          Repository
	Appearances   int    // 在该 period 榜上出现的不同日期数
	BestRank      int    // 历史最佳名次(最小值)
	PeakStarDelta int    // 上榜期间的最高单期 star 增量
	FirstRanked   string // 首次上榜日期 YYYY-MM-DD
	LastRanked    string // 最近上榜日期 YYYY-MM-DD
}

// ListArchive 列出在指定 period 榜单上出现过的所有仓库,按上榜次数降序
// (同数按最佳名次升序、再按 full_name)聚合分页。日期为字符串,字典序即时间序。
func (d *DB) ListArchive(period string, limit, offset int) ([]ArchiveEntry, error) {
	rows, err := d.db.Query(`
SELECT COUNT(*), MIN(tr.rank), MAX(tr.star_delta), MIN(tr.period_date), MAX(tr.period_date), `+repoColsR+`
FROM trending_rankings tr JOIN repositories r ON r.id = tr.repository_id
WHERE tr.period=?
GROUP BY tr.repository_id
ORDER BY COUNT(*) DESC, MIN(tr.rank), r.full_name
LIMIT ? OFFSET ?`, period, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ArchiveEntry
	for rows.Next() {
		var e ArchiveEntry
		var archived, active int
		if err := rows.Scan(
			&e.Appearances, &e.BestRank, &e.PeakStarDelta, &e.FirstRanked, &e.LastRanked,
			&e.Repo.ID, &e.Repo.GitHubID, &e.Repo.NodeID, &e.Repo.FullName, &e.Repo.Owner, &e.Repo.Name,
			&e.Repo.Description, &e.Repo.Language, &e.Repo.Homepage, &e.Repo.HTMLURL, &e.Repo.OwnerAvatar,
			&e.Repo.Stars, &e.Repo.Forks, &e.Repo.OpenIssues, &e.Repo.Watchers,
			&archived, &active, &e.Repo.RepoCreatedAt, &e.Repo.FirstSeenAt, &e.Repo.LastSyncedAt,
		); err != nil {
			return nil, err
		}
		e.Repo.IsArchived = archived == 1
		e.Repo.IsActive = active == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountArchive 返回在指定 period 榜单上出现过的不同仓库数。
func (d *DB) CountArchive(period string) (int, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(DISTINCT repository_id) FROM trending_rankings WHERE period=?`, period).Scan(&n)
	return n, err
}
