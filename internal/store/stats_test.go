package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	db := newTestDB(t)

	empty, err := db.Stats()
	require.NoError(t, err)
	require.Equal(t, 0, empty.ActiveRepos)
	require.Equal(t, "", empty.LatestRankingDate)

	id1, err := db.UpsertRepository(Repository{GitHubID: 1, NodeID: "R1", FullName: "a/a", Owner: "alice", Name: "a", HTMLURL: "u", Language: "Go"})
	require.NoError(t, err)
	_, err = db.UpsertRepository(Repository{GitHubID: 2, NodeID: "R2", FullName: "b/b", Owner: "bob", Name: "b", HTMLURL: "u", Language: "Rust"})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(1, 100, 0, 0, 0, "2026-06-10T00:00:00Z"))
	require.NoError(t, db.UpdateRepositoryMetrics(2, 200, 0, 0, 0, "2026-06-11T00:00:00Z"))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id1, Date: "2026-06-09", Stars: 90, StarDelta: 10}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id1, Date: "2026-06-10", Stars: 100, StarDelta: 10}))
	require.NoError(t, db.SetRepositoryTopics(id1, []string{"ai", "cli"}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id1, Rank: 1, Score: 1, StarDelta: 10, Language: "Go"}}))

	s, err := db.Stats()
	require.NoError(t, err)
	require.Equal(t, 2, s.ActiveRepos)
	require.Equal(t, 2, s.TotalSnapshots)
	require.Equal(t, 2, s.Languages)
	require.Equal(t, 2, s.Topics)
	require.Equal(t, 1, s.Developers)
	require.Equal(t, "2026-06-10", s.LatestRankingDate)
	require.Equal(t, "2026-06-11T00:00:00Z", s.LastSyncedAt)
}
