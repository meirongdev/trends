package store

import "database/sql"

// RankedRepository 是榜单项 + 仓库展示信息(JOIN 结果)。
type RankedRepository struct {
	Rank      int
	Score     float64
	StarDelta int
	Repo      Repository
}

// repoColsR 是带 r. 前缀、可用于 JOIN 的仓库列清单,顺序与 scanInto 一致。
const repoColsR = `r.id, r.github_id, r.node_id, r.full_name, r.owner, r.name,
       COALESCE(r.description,''), COALESCE(r.language,''), COALESCE(r.homepage,''),
       r.html_url, COALESCE(r.owner_avatar,''), r.stars, r.forks, r.open_issues, r.watchers,
       r.is_archived, r.is_active, COALESCE(r.repo_created_at,''), r.first_seen_at, COALESCE(r.last_synced_at,'')`

// LatestRankingDate 返回某 period 已物化的最新 period_date;无榜单时 ok=false。
func (d *DB) LatestRankingDate(period string) (string, bool, error) {
	var date sql.NullString
	if err := d.db.QueryRow(`SELECT MAX(period_date) FROM trending_rankings WHERE period=?`, period).Scan(&date); err != nil {
		return "", false, err
	}
	if !date.Valid {
		return "", false, nil
	}
	return date.String, true, nil
}

// CountRankings 返回某 period+date(可选语言过滤)的榜单条数。
func (d *DB) CountRankings(period, date, language string) (int, error) {
	q := `SELECT COUNT(*) FROM trending_rankings tr JOIN repositories r ON r.id=tr.repository_id WHERE tr.period=? AND tr.period_date=?`
	args := []any{period, date}
	if language != "" {
		q += ` AND r.language=?`
		args = append(args, language)
	}
	var n int
	err := d.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// ListRankings 返回某 period+date(可选语言过滤)的榜单项 + 仓库信息,按 rank 升序分页。
func (d *DB) ListRankings(period, date, language string, limit, offset int) ([]RankedRepository, error) {
	q := `SELECT tr.rank, tr.score, tr.star_delta, ` + repoColsR + `
FROM trending_rankings tr JOIN repositories r ON r.id = tr.repository_id
WHERE tr.period=? AND tr.period_date=?`
	args := []any{period, date}
	if language != "" {
		q += ` AND r.language=?`
		args = append(args, language)
	}
	q += ` ORDER BY tr.rank LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RankedRepository
	for rows.Next() {
		var rr RankedRepository
		var archived, active int
		if err := rows.Scan(
			&rr.Rank, &rr.Score, &rr.StarDelta,
			&rr.Repo.ID, &rr.Repo.GitHubID, &rr.Repo.NodeID, &rr.Repo.FullName, &rr.Repo.Owner, &rr.Repo.Name,
			&rr.Repo.Description, &rr.Repo.Language, &rr.Repo.Homepage, &rr.Repo.HTMLURL, &rr.Repo.OwnerAvatar,
			&rr.Repo.Stars, &rr.Repo.Forks, &rr.Repo.OpenIssues, &rr.Repo.Watchers,
			&archived, &active, &rr.Repo.RepoCreatedAt, &rr.Repo.FirstSeenAt, &rr.Repo.LastSyncedAt,
		); err != nil {
			return nil, err
		}
		rr.Repo.IsArchived = archived == 1
		rr.Repo.IsActive = active == 1
		out = append(out, rr)
	}
	return out, rows.Err()
}

// LanguageCount 是某语言下的活跃仓库数。
type LanguageCount struct {
	Language string
	Count    int
}

// LanguageCounts 返回各语言的活跃仓库数,按数量降序、同数按语言名升序;排除空语言。
func (d *DB) LanguageCounts() ([]LanguageCount, error) {
	rows, err := d.db.Query(`
SELECT language, COUNT(*) FROM repositories
WHERE is_active=1 AND language IS NOT NULL AND language <> ''
GROUP BY language
ORDER BY COUNT(*) DESC, language`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LanguageCount
	for rows.Next() {
		var lc LanguageCount
		if err := rows.Scan(&lc.Language, &lc.Count); err != nil {
			return nil, err
		}
		out = append(out, lc)
	}
	return out, rows.Err()
}

// Health 是 /healthz 暴露的运行信息。
type Health struct {
	LastSyncedAt string
	ActiveRepos  int
}

// HealthInfo 返回最近一次采集同步时间与活跃仓库数(无数据时分别为 ""/0)。
func (d *DB) HealthInfo() (Health, error) {
	var h Health
	var last sql.NullString
	if err := d.db.QueryRow(`SELECT MAX(last_synced_at) FROM repositories`).Scan(&last); err != nil {
		return h, err
	}
	h.LastSyncedAt = last.String
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM repositories WHERE is_active=1`).Scan(&h.ActiveRepos); err != nil {
		return h, err
	}
	return h, nil
}
