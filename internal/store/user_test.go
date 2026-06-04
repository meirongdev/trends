package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpsertUser(t *testing.T) {
	db := newTestDB(t)
	id1, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "42", Login: "octocat", Email: "o@x.com", AvatarURL: "av1"})
	require.NoError(t, err)
	require.NotZero(t, id1)

	id2, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "42", Login: "octocat-renamed", Email: "o@x.com", AvatarURL: "av2"})
	require.NoError(t, err)
	require.Equal(t, id1, id2)

	var login, avatar string
	require.NoError(t, db.db.QueryRow(`SELECT login, avatar_url FROM users WHERE id=?`, id1).Scan(&login, &avatar))
	require.Equal(t, "octocat-renamed", login)
	require.Equal(t, "av2", avatar)

	id3, err := db.UpsertUser(User{Provider: "google", ProviderUserID: "42", Login: "alice", AvatarURL: "av3"})
	require.NoError(t, err)
	require.NotEqual(t, id1, id3)
}
