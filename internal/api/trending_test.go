package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func seedRanking(t *testing.T, db *store.DB, gid int64, node, full, lang string, stars, rank, delta int, score float64, date string) {
	t.Helper()
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, HTMLURL: "https://gh/" + full, Language: lang, Description: "d",
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, 0, 0, 0, date+"T00:00:00Z"))
	existing := readRankings(t, db, "daily", date)
	require.NoError(t, db.ReplaceRankings("daily", date, append(existing, store.Ranking{
		RepositoryID: id, Rank: rank, Score: score, StarDelta: delta, Language: lang,
	})))
}

func readRankings(t *testing.T, db *store.DB, period, date string) []store.Ranking {
	t.Helper()
	rows, err := db.SQL().Query(`SELECT repository_id, rank, score, star_delta, COALESCE(language,'') FROM trending_rankings WHERE period=? AND period_date=? ORDER BY rank`, period, date)
	require.NoError(t, err)
	defer rows.Close()
	var out []store.Ranking
	for rows.Next() {
		var r store.Ranking
		require.NoError(t, rows.Scan(&r.RepositoryID, &r.Rank, &r.Score, &r.StarDelta, &r.Language))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

type trendingResp struct {
	Period  string `json:"period"`
	Date    string `json:"date"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Total   int    `json:"total"`
	Items   []struct {
		Rank       int     `json:"rank"`
		Score      float64 `json:"score"`
		StarDelta  int     `json:"star_delta"`
		Repository struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			Language string `json:"language"`
			Stars    int    `json:"stars"`
		} `json:"repository"`
	} `json:"items"`
}

func TestTrendingDefaultsToLatestDate(t *testing.T) {
	s, db := newTestServer(t)
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-09")
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1200, 1, 250, 0.95, "2026-06-10")

	rec := doGET(t, s, "/api/v1/trending")
	require.Equal(t, http.StatusOK, rec.Code)
	var body trendingResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "daily", body.Period)
	require.Equal(t, "2026-06-10", body.Date)
	require.Equal(t, 1, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, 1, body.Items[0].Rank)
	require.Equal(t, "a/go1", body.Items[0].Repository.FullName)
	require.Equal(t, 250, body.Items[0].StarDelta)
}

func TestTrendingLanguageFilterAndPaging(t *testing.T) {
	s, db := newTestServer(t)
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-10")
	seedRanking(t, db, 2, "R2", "a/go2", "Go", 800, 2, 100, 0.5, "2026-06-10")
	seedRanking(t, db, 3, "R3", "a/rust1", "Rust", 500, 3, 50, 0.3, "2026-06-10")

	rec := doGET(t, s, "/api/v1/trending?language=Go&date=2026-06-10")
	var body trendingResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.Total)
	require.Len(t, body.Items, 2)

	rec = doGET(t, s, "/api/v1/trending?date=2026-06-10&per_page=1&page=2")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 3, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, 2, body.Items[0].Rank)
}

func TestTrendingRejectsBadPeriod(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/trending?period=hourly")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTrendingRejectsBadDate(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/trending?date=foobar")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTrendingAcceptsYearly(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/trending?period=yearly")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestTrendingEmptyWhenNoRankings(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/trending")
	require.Equal(t, http.StatusOK, rec.Code)
	var body trendingResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Total)
	require.NotNil(t, body.Items)
	require.Len(t, body.Items, 0)
}
