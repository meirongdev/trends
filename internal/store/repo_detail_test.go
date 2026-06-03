package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetRepositoryByID(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	got, ok, err := db.GetRepositoryByID(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, id, got.ID)
	require.Equal(t, "octocat/hello", got.FullName)

	_, ok, err = db.GetRepositoryByID(99999)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestBestDailyRank(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	_, ok, err := db.BestDailyRank(id)
	require.NoError(t, err)
	require.False(t, ok) // 没上过榜

	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: id, Rank: 5, Score: 0.5, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 2, Score: 0.8, StarDelta: 20, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 30, Language: "Go"}}))

	best, ok, err := db.BestDailyRank(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2, best) // daily 最小 rank(weekly 的 1 不算)
}
