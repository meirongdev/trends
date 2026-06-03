package store

// Ranking 是 trending_rankings 的一行(物化的榜单项)。
type Ranking struct {
	RepositoryID int64
	Rank         int
	Score        float64
	StarDelta    int
	Language     string
}

// ReplaceRankings 在单个事务里,用 rows 整体替换 (period, periodDate) 的榜单:
// 先删该 period+date 的旧行,再插入新行。对重跑幂等;rows 为空则仅清空。
func (d *DB) ReplaceRankings(period, periodDate string, rows []Ranking) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM trending_rankings WHERE period=? AND period_date=?`,
		period, periodDate); err != nil {
		tx.Rollback()
		return err
	}
	for _, r := range rows {
		if _, err := tx.Exec(`
INSERT INTO trending_rankings
  (period, period_date, repository_id, rank, score, star_delta, language)
VALUES (?,?,?,?,?,?,?)`,
			period, periodDate, r.RepositoryID, r.Rank, r.Score, r.StarDelta, r.Language); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
