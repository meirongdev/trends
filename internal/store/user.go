package store

// User 是经 OAuth 登录的用户。
type User struct {
	ID             int64
	Provider       string
	ProviderUserID string
	Login          string
	Email          string
	AvatarURL      string
	CreatedAt      string
}

// UpsertUser 按 (provider, provider_user_id) 插入或更新登录/邮箱/头像,返回内部 id。
// created_at 仅插入时设置。
func (d *DB) UpsertUser(u User) (int64, error) {
	_, err := d.db.Exec(`
INSERT INTO users (provider, provider_user_id, login, email, avatar_url, created_at)
VALUES (?,?,?,?,?,?)
ON CONFLICT(provider, provider_user_id) DO UPDATE SET
  login      = excluded.login,
  email      = excluded.email,
  avatar_url = excluded.avatar_url`,
		u.Provider, u.ProviderUserID, u.Login, u.Email, u.AvatarURL, nowUTC())
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.QueryRow(
		`SELECT id FROM users WHERE provider=? AND provider_user_id=?`, u.Provider, u.ProviderUserID,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}
