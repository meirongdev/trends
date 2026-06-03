package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// seedRepoWithMetrics 建仓库并设其指标,返回内部 id。
func seedRepoWithMetrics(t *testing.T, db *store.DB, gid int64, node, full, lang string, stars, forks, issues int) int64 {
	t.Helper()
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, Description: "d", HTMLURL: "https://gh/" + full, Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, forks, issues, 0, "2026-06-10T00:00:00Z"))
	return id
}

func TestRepositoryDetailReturnsRepoAndBestRank(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{{RepositoryID: id, Rank: 3, Score: 0.7, StarDelta: 40, Language: "Go"}}))

	rec := doGET(t, s, "/api/v1/repositories/1")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		Stars         int    `json:"stars"`
		Forks         int    `json:"forks"`
		BestDailyRank *int   `json:"best_daily_rank"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, id, body.ID)
	require.Equal(t, "a/x", body.FullName)
	require.Equal(t, 1000, body.Stars)
	require.Equal(t, 50, body.Forks)
	require.NotNil(t, body.BestDailyRank)
	require.Equal(t, 3, *body.BestDailyRank)
}

func TestRepositoryDetailNotFound(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/99999")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRepositoryDetailBadID(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/abc")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRepositoryDetailNullBestRankWhenNeverRanked(t *testing.T) {
	s, db := newTestServer(t)
	_ = seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	rec := doGET(t, s, "/api/v1/repositories/1")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		BestDailyRank *int `json:"best_daily_rank"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Nil(t, body.BestDailyRank) // 从未上榜 → null
}
