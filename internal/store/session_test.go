package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessions(t *testing.T) {
	db := newTestDB(t)
	uid, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "1", Login: "u", AvatarURL: "a"})
	require.NoError(t, err)

	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	require.NoError(t, db.CreateSession("hash-valid", uid, future))

	u, ok, err := db.SessionUser("hash-valid")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "u", u.Login)
	require.Equal(t, uid, u.ID)

	_, ok, err = db.SessionUser("nope")
	require.NoError(t, err)
	require.False(t, ok)

	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	require.NoError(t, db.CreateSession("hash-expired", uid, past))
	_, ok, err = db.SessionUser("hash-expired")
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, db.DeleteSession("hash-valid"))
	_, ok, _ = db.SessionUser("hash-valid")
	require.False(t, ok)

	n, err := db.DeleteExpiredSessions()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}
