package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) (*Server, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewServer(db), db
}

func doGET(t *testing.T, s *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func TestHealthzReportsStatus(t *testing.T) {
	s, db := newTestServer(t)
	_, err := db.UpsertRepository(store.Repository{GitHubID: 1, NodeID: "R1", FullName: "a/1", Owner: "a", Name: "1", HTMLURL: "u", Language: "Go"})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(1, 100, 0, 0, 0, "2026-06-10T00:00:00Z"))

	rec := doGET(t, s, "/healthz")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body struct {
		Status       string `json:"status"`
		LastSyncedAt string `json:"last_synced_at"`
		ActiveRepos  int    `json:"active_repos"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ok", body.Status)
	require.Equal(t, 1, body.ActiveRepos)
	require.Equal(t, "2026-06-10T00:00:00Z", body.LastSyncedAt)
}
