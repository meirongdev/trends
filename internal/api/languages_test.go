package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLanguagesReturnsLeaderboardCounts(t *testing.T) {
	s, db := newTestServer(t)
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-10")
	seedRanking(t, db, 2, "R2", "a/go2", "Go", 800, 2, 100, 0.5, "2026-06-10")
	seedRanking(t, db, 3, "R3", "a/rust1", "Rust", 500, 3, 50, 0.3, "2026-06-10")

	rec := doGET(t, s, "/api/v1/languages") // 默认 daily + 最新日期
	require.Equal(t, http.StatusOK, rec.Code)
	var body []struct {
		Language string `json:"language"`
		Count    int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, "Go", body[0].Language)
	require.Equal(t, 2, body[0].Count)
	require.Equal(t, "Rust", body[1].Language)
	require.Equal(t, 1, body[1].Count)
}

func TestLanguagesEmptyReturnsEmptyArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/languages")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[]\n", rec.Body.String()) // 空数组而非 null
}

func TestLanguagesRejectsBadPeriod(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/languages?period=hourly")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
