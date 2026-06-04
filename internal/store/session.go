package store

import (
	"database/sql"
	"errors"
)

// CreateSession 写入一条会话(id 为 token 的 sha256;expiresAt 为 RFC3339)。
func (d *DB) CreateSession(tokenHash string, userID int64, expiresAt string) error {
	_, err := d.db.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?,?,?,?)`,
		tokenHash, userID, nowUTC(), expiresAt)
	return err
}

// SessionUser 按 token 的 sha256 取未过期会话对应的用户;不存在或已过期 → ok=false。
func (d *DB) SessionUser(tokenHash string) (User, bool, error) {
	var u User
	err := d.db.QueryRow(`
SELECT u.id, u.provider, u.provider_user_id, u.login, COALESCE(u.email,''), COALESCE(u.avatar_url,''), u.created_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.id = ? AND s.expires_at > ?`, tokenHash, nowUTC()).
		Scan(&u.ID, &u.Provider, &u.ProviderUserID, &u.Login, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return u, true, nil
}

// DeleteSession 删除指定会话(登出)。
func (d *DB) DeleteSession(tokenHash string) error {
	_, err := d.db.Exec(`DELETE FROM sessions WHERE id=?`, tokenHash)
	return err
}

// DeleteExpiredSessions 删除所有已过期会话,返回删除条数。
func (d *DB) DeleteExpiredSessions() (int64, error) {
	res, err := d.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
