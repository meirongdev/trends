package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meirongdev/trends/internal/auth"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// fakeProviderServer 返回一个假 OAuth provider(/token + /userinfo)。
func fakeProviderServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "bearer"})
		case "/userinfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":55,"login":"alice","avatar_url":"av"}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

// serverWithGithub 返回一个挂了「github」假 provider 的 Server。
func serverWithGithub(t *testing.T, providerURL string) (*Server, *store.DB) {
	t.Helper()
	s, db := newTestServer(t)
	s.providers = map[string]*auth.Provider{
		"github": auth.NewProvider(auth.ProviderSpec{
			Name: "github", ClientID: "cid", ClientSecret: "sec",
			RedirectURL: "http://app/api/v1/auth/callback?provider=github",
			AuthURL:     providerURL + "/authorize", TokenURL: providerURL + "/token",
			UserInfoURL: providerURL + "/userinfo", Parse: auth.GithubParse,
		}),
	}
	return s, db
}

// loggedInCookie 建一个用户与未过期会话,返回带原始 token 的 session cookie。
func loggedInCookie(t *testing.T, db *store.DB) *http.Cookie {
	t.Helper()
	uid, err := db.UpsertUser(store.User{Provider: "github", ProviderUserID: "1", Login: "u", AvatarURL: "a"})
	require.NoError(t, err)
	raw, err := auth.RandomToken(32)
	require.NoError(t, err)
	require.NoError(t, db.CreateSession(auth.HashToken(raw), uid, "2999-01-01T00:00:00Z"))
	return &http.Cookie{Name: sessionCookieName, Value: raw}
}

func TestAuthLoginRedirects(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, _ := serverWithGithub(t, prov.URL)

	rec := doGET(t, s, "/api/v1/auth/login?provider=github")
	require.Equal(t, http.StatusFound, rec.Code)
	loc := rec.Header().Get("Location")
	require.True(t, strings.HasPrefix(loc, prov.URL+"/authorize"))
	require.Contains(t, loc, "state=")
	require.Contains(t, rec.Header().Get("Set-Cookie"), stateCookieName)
}

func TestAuthLoginUnconfigured(t *testing.T) {
	s, _ := newTestServer(t) // 无 providers
	rec := doGET(t, s, "/api/v1/auth/login?provider=github")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAuthCallbackHappyPath(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, db := serverWithGithub(t, prov.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?provider=github&code=c&state=st", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "st"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/submit", rec.Header().Get("Location"))
	var sessionSet bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			sessionSet = true
		}
	}
	require.True(t, sessionSet, "session cookie should be set")

	var n int
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM users WHERE provider='github' AND provider_user_id='55'`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestAuthCallbackBadState(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, _ := serverWithGithub(t, prov.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?provider=github&code=c&state=evil", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "st"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthMeAndLogout(t *testing.T) {
	s, db := newTestServer(t)
	cookie := loggedInCookie(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var me struct {
		User *struct {
			Login string `json:"login"`
		} `json:"user"`
		Providers []string `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &me))
	require.NotNil(t, me.User)
	require.Equal(t, "u", me.User.Login)

	lreq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	lreq.AddCookie(cookie)
	lrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec, lreq)
	require.Equal(t, http.StatusNoContent, lrec.Code)

	_, ok, _ := db.SessionUser(auth.HashToken(cookie.Value))
	require.False(t, ok) // 会话已删
}

func TestAuthMeAnonymous(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, _ := serverWithGithub(t, prov.URL)
	rec := doGET(t, s, "/api/v1/auth/me")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"user":null`)
	require.Contains(t, rec.Body.String(), `"github"`) // providers 含已配置项
}
