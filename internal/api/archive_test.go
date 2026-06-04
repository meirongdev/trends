package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	s, db := newTestServer(t)
	a, err := db.UpsertRepository(store.Repository{GitHubID: 1, NodeID: "RA", FullName: "alice/a", Owner: "alice", Name: "a", HTMLURL: "u", Language: "Go", OwnerAvatar: "av-a"})
	require.NoError(t, err)
	b, err := db.UpsertRepository(store.Repository{GitHubID: 2, NodeID: "RB", FullName: "bob/b", Owner: "bob", Name: "b", HTMLURL: "u"})
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []store.Ranking{{RepositoryID: a, Rank: 3, Score: 1, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{
		{RepositoryID: a, Rank: 1, Score: 1, StarDelta: 30, Language: "Go"},
		{RepositoryID: b, Rank: 2, Score: 0.5, StarDelta: 5, Language: "Rust"},
	}))

	rec := doGET(t, s, "/api/v1/archive")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Period string `json:"period"`
		Total  int    `json:"total"`
		Items  []struct {
			Repository struct {
				FullName string `json:"full_name"`
				Owner    string `json:"owner"`
			} `json:"repository"`
			Appearances   int    `json:"appearances"`
			BestRank      int    `json:"best_rank"`
			PeakStarDelta int    `json:"peak_star_delta"`
			FirstRanked   string `json:"first_ranked"`
			LastRanked    string `json:"last_ranked"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "daily", body.Period)
	require.Equal(t, 2, body.Total)
	require.Len(t, body.Items, 2)
	require.Equal(t, "alice/a", body.Items[0].Repository.FullName)
	require.Equal(t, 2, body.Items[0].Appearances)
	require.Equal(t, 1, body.Items[0].BestRank)
	require.Equal(t, 30, body.Items[0].PeakStarDelta)
	require.Equal(t, "2026-06-09", body.Items[0].FirstRanked)
	require.Equal(t, "2026-06-10", body.Items[0].LastRanked)
	require.Equal(t, "bob/b", body.Items[1].Repository.FullName)
}

func TestArchiveEmptyIsArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/archive")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Items)
	require.Len(t, body.Items, 0)
}

func TestArchiveBadPeriod(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/archive?period=hourly")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
