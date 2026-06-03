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

func TestRepositorySnapshotsRangeAndOrder(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	for _, s := range []Snapshot{
		{RepositoryID: id, Date: "2026-06-08", Stars: 100, Forks: 5, OpenIssues: 1, Watchers: 3, StarDelta: 10},
		{RepositoryID: id, Date: "2026-06-09", Stars: 130, Forks: 6, OpenIssues: 2, Watchers: 4, StarDelta: 30},
		{RepositoryID: id, Date: "2026-06-10", Stars: 150, Forks: 7, OpenIssues: 2, Watchers: 4, StarDelta: 20},
	} {
		require.NoError(t, db.InsertSnapshot(s))
	}

	all, err := db.RepositorySnapshots(id, "", "")
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, "2026-06-08", all[0].Date)
	require.Equal(t, 100, all[0].Stars)
	require.Equal(t, "2026-06-10", all[2].Date)

	rng, err := db.RepositorySnapshots(id, "2026-06-09", "2026-06-10")
	require.NoError(t, err)
	require.Len(t, rng, 2)
	require.Equal(t, "2026-06-09", rng[0].Date)
}

func TestRepositoryRankingHistory(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: id, Rank: 5, Score: 0.5, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 2, Score: 0.8, StarDelta: 20, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 30, Language: "Go"}}))

	hist, err := db.RepositoryRankingHistory(id)
	require.NoError(t, err)
	require.Len(t, hist, 3)
	require.Equal(t, "2026-06-10", hist[0].Date) // 最新日期在前

	found := map[string]RankingHistory{}
	for _, h := range hist {
		found[h.Period+"@"+h.Date] = h
	}
	require.Equal(t, 2, found["daily@2026-06-10"].Rank)
	require.Equal(t, 1, found["weekly@2026-06-10"].Rank)
	require.Equal(t, 5, found["daily@2026-06-09"].Rank)
}
