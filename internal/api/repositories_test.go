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

func TestRepositorySnapshotsEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id, Date: "2026-06-09", Stars: 130, StarDelta: 30}))
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id, Date: "2026-06-10", Stars: 150, StarDelta: 20}))

	rec := doGET(t, s, "/api/v1/repositories/1/snapshots")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		RepositoryID int64 `json:"repository_id"`
		Snapshots    []struct {
			Date      string `json:"date"`
			Stars     int    `json:"stars"`
			StarDelta int    `json:"star_delta"`
		} `json:"snapshots"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, id, body.RepositoryID)
	require.Len(t, body.Snapshots, 2)
	require.Equal(t, "2026-06-09", body.Snapshots[0].Date)
	require.Equal(t, 130, body.Snapshots[0].Stars)

	rec = doGET(t, s, "/api/v1/repositories/1/snapshots?from=2026-06-10")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Snapshots, 1)

	rec = doGET(t, s, "/api/v1/repositories/1/snapshots?from=bad")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRepositoryRankingsEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{{RepositoryID: id, Rank: 2, Score: 0.8, StarDelta: 20, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []store.Ranking{{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 30, Language: "Go"}}))

	rec := doGET(t, s, "/api/v1/repositories/1/rankings")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		RepositoryID int64 `json:"repository_id"`
		Rankings     []struct {
			Period    string  `json:"period"`
			Date      string  `json:"date"`
			Rank      int     `json:"rank"`
			Score     float64 `json:"score"`
			StarDelta int     `json:"star_delta"`
		} `json:"rankings"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, id, body.RepositoryID)
	require.Len(t, body.Rankings, 2)
}

func TestRepositorySnapshotsBadID(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/abc/snapshots")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
