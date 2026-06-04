package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestDevelopersRanking(t *testing.T) {
	s, db := newTestServer(t)
	a, err := db.UpsertRepository(store.Repository{GitHubID: 1, NodeID: "RA", FullName: "alice/a", Owner: "alice", Name: "a", HTMLURL: "u", OwnerAvatar: "av-a"})
	require.NoError(t, err)
	b, err := db.UpsertRepository(store.Repository{GitHubID: 2, NodeID: "RB", FullName: "bob/b", Owner: "bob", Name: "b", HTMLURL: "u"})
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []store.Ranking{{RepositoryID: a, Rank: 1, Score: 1, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{
		{RepositoryID: a, Rank: 1, Score: 1, StarDelta: 20, Language: "Go"},
		{RepositoryID: b, Rank: 2, Score: 0.5, StarDelta: 5, Language: "Go"},
	}))

	rec := doGET(t, s, "/api/v1/developers")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Period string `json:"period"`
		Total  int    `json:"total"`
		Items  []struct {
			Login       string `json:"login"`
			Avatar      string `json:"avatar"`
			Appearances int    `json:"appearances"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "daily", body.Period)
	require.Equal(t, 2, body.Total)
	require.Len(t, body.Items, 2)
	require.Equal(t, "alice", body.Items[0].Login)
	require.Equal(t, 2, body.Items[0].Appearances)
	require.Equal(t, "av-a", body.Items[0].Avatar)
}

func TestDevelopersEmptyIsArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/developers")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Items)
	require.Len(t, body.Items, 0)
}

func TestDevelopersBadPeriod(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/developers?period=hourly")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
