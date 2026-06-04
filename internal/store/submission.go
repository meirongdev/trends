package store

type Submission struct {
	ID          int64
	FullName    string
	Status      string
	SubmittedIP string
	Note        string
	CreatedAt   string
}

// InsertSubmission 记录一条 pending 提交(关联登录用户),返回 id。
// 传 userID=0 表示匿名提交,user_id 存为 NULL。
func (d *DB) InsertSubmission(fullName, ip string, userID int64) (int64, error) {
	var uid interface{}
	if userID != 0 {
		uid = userID
	}
	res, err := d.db.Exec(
		`INSERT INTO submissions (full_name, status, submitted_ip, user_id, created_at) VALUES (?, 'pending', ?, ?, ?)`,
		fullName, ip, uid, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListPendingSubmissions 返回最多 limit 条 pending 提交(按 id 升序)。
func (d *DB) ListPendingSubmissions(limit int) ([]Submission, error) {
	rows, err := d.db.Query(`
SELECT id, full_name, status, COALESCE(submitted_ip,''), COALESCE(note,''), created_at
FROM submissions WHERE status='pending' ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Submission
	for rows.Next() {
		var s Submission
		if err := rows.Scan(&s.ID, &s.FullName, &s.Status, &s.SubmittedIP, &s.Note, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// MarkSubmission 更新提交状态(accepted/rejected)与备注。
func (d *DB) MarkSubmission(id int64, status, note string) error {
	_, err := d.db.Exec(`UPDATE submissions SET status=?, note=? WHERE id=?`, status, note, id)
	return err
}
