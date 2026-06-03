package store

// searchWhere 构造搜索的 WHERE 子句与参数:在活跃仓库中按 full_name/description 模糊匹配,
// 可选语言过滤。q 作为 LIKE 值绑定(防注入)。
func searchWhere(q, language string) (string, []any) {
	where := ` WHERE is_active=1 AND (full_name LIKE ? OR COALESCE(description,'') LIKE ?)`
	pat := "%" + q + "%"
	args := []any{pat, pat}
	if language != "" {
		where += ` AND language=?`
		args = append(args, language)
	}
	return where, args
}

// SearchRepositoriesCount 返回匹配的活跃仓库总数。
func (d *DB) SearchRepositoriesCount(q, language string) (int, error) {
	where, args := searchWhere(q, language)
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM repositories`+where, args...).Scan(&n)
	return n, err
}

// SearchRepositoriesByText 返回匹配的活跃仓库,按 stars 降序、同 stars 按 full_name,分页。
func (d *DB) SearchRepositoriesByText(q, language string, limit, offset int) ([]Repository, error) {
	where, args := searchWhere(q, language)
	args = append(args, limit, offset)
	rows, err := d.db.Query(`SELECT `+repoSelectColumns+` FROM repositories`+where+` ORDER BY stars DESC, full_name LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Repository
	for rows.Next() {
		r, err := scanRepoRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
