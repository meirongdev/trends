package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

type statsBody struct {
	ActiveRepos       int    `json:"active_repos"`
	TotalSnapshots    int    `json:"total_snapshots"`
	Languages         int    `json:"languages"`
	Topics            int    `json:"topics"`
	Developers        int    `json:"developers"`
	LatestRankingDate string `json:"latest_ranking_date"`
	LastSyncedAt      string `json:"last_synced_at"`
}

func TestStatsEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	id1, err := db.UpsertRepository(store.Repository{GitHubID: 1, NodeID: "R1", FullName: "a/a", Owner: "alice", Name: "a", HTMLURL: "u", Language: "Go"})
	require.NoError(t, err)
	_, err = db.UpsertRepository(store.Repository{GitHubID: 2, NodeID: "R2", FullName: "b/b", Owner: "bob", Name: "b", HTMLURL: "u", Language: "Rust"})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(1, 100, 0, 0, 0, "2026-06-10T00:00:00Z"))
	require.NoError(t, db.UpdateRepositoryMetrics(2, 200, 0, 0, 0, "2026-06-11T00:00:00Z"))
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id1, Date: "2026-06-10", Stars: 100, StarDelta: 10}))
	require.NoError(t, db.SetRepositoryTopics(id1, []string{"ai", "cli"}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{{RepositoryID: id1, Rank: 1, Score: 1, StarDelta: 10, Language: "Go"}}))

	rec := doGET(t, s, "/api/v1/stats")
	require.Equal(t, http.StatusOK, rec.Code)
	var body statsBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.ActiveRepos)
	require.Equal(t, 1, body.TotalSnapshots)
	require.Equal(t, 2, body.Languages)
	require.Equal(t, 2, body.Topics)
	require.Equal(t, 1, body.Developers)
	require.Equal(t, "2026-06-10", body.LatestRankingDate)
	require.Equal(t, "2026-06-11T00:00:00Z", body.LastSyncedAt)
}

func TestStatsEndpointEmpty(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/stats")
	require.Equal(t, http.StatusOK, rec.Code)
	var body statsBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.ActiveRepos)
	require.Equal(t, "", body.LatestRankingDate)
}
