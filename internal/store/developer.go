package store

// Developer 是按上榜次数聚合的开发者(owner)。
type Developer struct {
	Login       string
	Avatar      string
	Appearances int
}

// ListDevelopers 按指定 period 榜上的累计上榜次数对 owner 聚合排名,降序分页。
func (d *DB) ListDevelopers(period string, limit, offset int) ([]Developer, error) {
	rows, err := d.db.Query(`
SELECT r.owner, COALESCE(MAX(r.owner_avatar), ''), COUNT(*)
FROM trending_rankings tr JOIN repositories r ON r.id = tr.repository_id
WHERE tr.period=?
GROUP BY r.owner
ORDER BY COUNT(*) DESC, r.owner
LIMIT ? OFFSET ?`, period, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Developer
	for rows.Next() {
		var dev Developer
		if err := rows.Scan(&dev.Login, &dev.Avatar, &dev.Appearances); err != nil {
			return nil, err
		}
		out = append(out, dev)
	}
	return out, rows.Err()
}

// CountDevelopers 返回在指定 period 榜上出现过的不同 owner 数。
func (d *DB) CountDevelopers(period string) (int, error) {
	var n int
	err := d.db.QueryRow(`
SELECT COUNT(*) FROM (
  SELECT 1 FROM trending_rankings tr JOIN repositories r ON r.id = tr.repository_id
  WHERE tr.period=? GROUP BY r.owner
)`, period).Scan(&n)
	return n, err
}
