package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func TestSubmissionAccepted(t *testing.T) {
	s, db := newTestServer(t)
	rec := postJSON(t, s, "/api/v1/submissions", `{"full_name":"octocat/hello"}`)
	require.Equal(t, http.StatusAccepted, rec.Code)

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Len(t, pend, 1)
	require.Equal(t, "octocat/hello", pend[0].FullName)
}

func TestSubmissionRejectsBadInput(t *testing.T) {
	s, _ := newTestServer(t)
	for _, bad := range []string{
		`{"full_name":"noslash"}`,
		`{"full_name":"a/b/c"}`,
		`{"full_name":""}`,
		`not json`,
	} {
		rec := postJSON(t, s, "/api/v1/submissions", bad)
		require.Equal(t, http.StatusBadRequest, rec.Code, bad)
	}
}

func TestSubmissionRateLimited(t *testing.T) {
	s, _ := newTestServer(t)
	s.submitLimiter = newRateLimiter(1, time.Hour) // 测试用低阈值
	require.Equal(t, http.StatusAccepted, postJSON(t, s, "/api/v1/submissions", `{"full_name":"a/b"}`).Code)
	require.Equal(t, http.StatusTooManyRequests, postJSON(t, s, "/api/v1/submissions", `{"full_name":"a/c"}`).Code)
}
