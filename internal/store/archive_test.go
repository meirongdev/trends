package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListArchive(t *testing.T) {
	db := newTestDB(t)
	a, err := db.UpsertRepository(Repository{GitHubID: 1, NodeID: "RA", FullName: "a/a", Owner: "alice", Name: "a", HTMLURL: "u", Language: "Go"})
	require.NoError(t, err)
	b, err := db.UpsertRepository(Repository{GitHubID: 2, NodeID: "RB", FullName: "b/b", Owner: "bob", Name: "b", HTMLURL: "u", Language: "Rust"})
	require.NoError(t, err)

	// A ranks on two daily dates; B on one daily date and one weekly date.
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: a, Rank: 3, Score: 1, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: a, Rank: 1, Score: 1, StarDelta: 30, Language: "Go"},
		{RepositoryID: b, Rank: 2, Score: 0.5, StarDelta: 5, Language: "Rust"},
	}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{{RepositoryID: b, Rank: 1, Score: 1, StarDelta: 50, Language: "Rust"}}))

	total, err := db.CountArchive("daily")
	require.NoError(t, err)
	require.Equal(t, 2, total)

	entries, err := db.ListArchive("daily", 25, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	require.Equal(t, "a/a", entries[0].Repo.FullName)
	require.Equal(t, 2, entries[0].Appearances)
	require.Equal(t, 1, entries[0].BestRank)
	require.Equal(t, 30, entries[0].PeakStarDelta)
	require.Equal(t, "2026-06-09", entries[0].FirstRanked)
	require.Equal(t, "2026-06-10", entries[0].LastRanked)

	require.Equal(t, "b/b", entries[1].Repo.FullName)
	require.Equal(t, 1, entries[1].Appearances)
	require.Equal(t, 2, entries[1].BestRank)

	// pagination
	page2, err := db.ListArchive("daily", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "b/b", page2[0].Repo.FullName)

	// period isolation: weekly archive has only b
	wtotal, err := db.CountArchive("weekly")
	require.NoError(t, err)
	require.Equal(t, 1, wtotal)
}
