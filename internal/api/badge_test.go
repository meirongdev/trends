package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestBadgeForRankedRepo(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 0, 0)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{
		{RepositoryID: id, Rank: 3, Score: 0.7, StarDelta: 40, Language: "Go"},
	}))

	rec := doGET(t, s, "/api/v1/repositories/1/badge.svg")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "image/svg+xml")
	require.NotEmpty(t, rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Body.String(), "rank #3")
	require.Contains(t, rec.Body.String(), "<svg")
}

func TestBadgeForUnrankedRepo(t *testing.T) {
	s, db := newTestServer(t)
	_ = seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 0, 0)
	rec := doGET(t, s, "/api/v1/repositories/1/badge.svg")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "unranked")
}

func TestBadgeForUnknownRepoIsPlaceholder(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/99999/badge.svg")
	require.Equal(t, http.StatusOK, rec.Code) // 占位,不 404(避免 README 破图)
	require.Contains(t, rec.Header().Get("Content-Type"), "image/svg+xml")
	require.True(t, strings.Contains(rec.Body.String(), "n/a"))
}
