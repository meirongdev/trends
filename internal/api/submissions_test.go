package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, s *Server, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func TestSubmissionRequiresLogin(t *testing.T) {
	s, _ := newTestServer(t)
	rec := postJSON(t, s, "/api/v1/submissions", `{"full_name":"octocat/hello"}`)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSubmissionAccepted(t *testing.T) {
	s, db := newTestServer(t)
	cookie := loggedInCookie(t, db)
	rec := postJSON(t, s, "/api/v1/submissions", `{"full_name":"octocat/hello"}`, cookie)
	require.Equal(t, http.StatusAccepted, rec.Code)

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Len(t, pend, 1)
	require.Equal(t, "octocat/hello", pend[0].FullName)

	var uid int64
	require.NoError(t, db.SQL().QueryRow(`SELECT user_id FROM submissions WHERE full_name='octocat/hello'`).Scan(&uid))
	require.NotZero(t, uid)
}

func TestSubmissionRejectsBadInput(t *testing.T) {
	s, db := newTestServer(t)
	cookie := loggedInCookie(t, db)
	for _, bad := range []string{
		`{"full_name":"noslash"}`,
		`{"full_name":"a/b/c"}`,
		`{"full_name":""}`,
		`not json`,
	} {
		rec := postJSON(t, s, "/api/v1/submissions", bad, cookie)
		require.Equal(t, http.StatusBadRequest, rec.Code, bad)
	}
}

func TestSubmissionRateLimited(t *testing.T) {
	s, db := newTestServer(t)
	s.submitLimiter = newRateLimiter(1, time.Hour) // 测试用低阈值(按用户)
	cookie := loggedInCookie(t, db)
	require.Equal(t, http.StatusAccepted, postJSON(t, s, "/api/v1/submissions", `{"full_name":"a/b"}`, cookie).Code)
	require.Equal(t, http.StatusTooManyRequests, postJSON(t, s, "/api/v1/submissions", `{"full_name":"a/c"}`, cookie).Code)
}
