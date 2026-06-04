package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func mkOwnedRepo(t *testing.T, db *DB, gid int64, node, full, owner string) int64 {
	t.Helper()
	id, err := db.UpsertRepository(Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: owner, Name: node,
		HTMLURL: "u", OwnerAvatar: "https://av/" + owner,
	})
	require.NoError(t, err)
	return id
}

func TestListDevelopersByDailyAppearances(t *testing.T) {
	db := newTestDB(t)
	aliceID := mkOwnedRepo(t, db, 1, "RA", "alice/a", "alice")
	bobID := mkOwnedRepo(t, db, 2, "RB", "bob/b", "bob")
	carolID := mkOwnedRepo(t, db, 3, "RC", "carol/c", "carol")

	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{
		{RepositoryID: aliceID, Rank: 1, Score: 1, StarDelta: 10, Language: "Go"},
	}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: aliceID, Rank: 1, Score: 1, StarDelta: 20, Language: "Go"},
		{RepositoryID: bobID, Rank: 2, Score: 0.5, StarDelta: 5, Language: "Go"},
	}))
	// carol 只在 weekly,不应计入 daily
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{
		{RepositoryID: carolID, Rank: 1, Score: 1, StarDelta: 30, Language: "Go"},
	}))

	devs, err := db.ListDevelopers("daily", 25, 0)
	require.NoError(t, err)
	require.Len(t, devs, 2)
	require.Equal(t, "alice", devs[0].Login)
	require.Equal(t, 2, devs[0].Appearances)
	require.Equal(t, "https://av/alice", devs[0].Avatar)
	require.Equal(t, "bob", devs[1].Login)
	require.Equal(t, 1, devs[1].Appearances)

	total, err := db.CountDevelopers("daily")
	require.NoError(t, err)
	require.Equal(t, 2, total)

	// 分页
	page2, err := db.ListDevelopers("daily", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "bob", page2[0].Login)
}
