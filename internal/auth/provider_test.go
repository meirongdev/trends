package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGithubParse(t *testing.T) {
	id, err := GithubParse([]byte(`{"id":123,"login":"octocat","avatar_url":"av","email":"o@x.com"}`))
	require.NoError(t, err)
	require.Equal(t, "github", id.Provider)
	require.Equal(t, "123", id.ProviderUserID)
	require.Equal(t, "octocat", id.Login)
	require.Equal(t, "av", id.AvatarURL)
	require.Equal(t, "o@x.com", id.Email)
}

func TestGoogleParse(t *testing.T) {
	id, err := GoogleParse([]byte(`{"sub":"sub-9","email":"a@b.com","name":"Alice","picture":"pic"}`))
	require.NoError(t, err)
	require.Equal(t, "google", id.Provider)
	require.Equal(t, "sub-9", id.ProviderUserID)
	require.Equal(t, "Alice", id.Login)
	require.Equal(t, "pic", id.AvatarURL)
}

func TestProviderFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-xyz", "token_type": "bearer"})
		case "/userinfo":
			require.Equal(t, "Bearer tok-xyz", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":7,"login":"flow","avatar_url":"a"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewProvider(ProviderSpec{
		Name: "github", ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://app/cb",
		AuthURL: srv.URL + "/authorize", TokenURL: srv.URL + "/token", UserInfoURL: srv.URL + "/userinfo",
		Parse: GithubParse,
	})

	u := p.AuthCodeURL("state123")
	require.True(t, strings.HasPrefix(u, srv.URL+"/authorize"))
	require.Contains(t, u, "client_id=cid")
	require.Contains(t, u, "state=state123")

	tok, err := p.Exchange(context.Background(), "code-abc")
	require.NoError(t, err)
	id, err := p.FetchIdentity(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, "7", id.ProviderUserID)
	require.Equal(t, "flow", id.Login)
}

func TestNewProvidersOnlyConfigured(t *testing.T) {
	ps := NewProviders(Config{
		BaseURL:        "http://localhost:8080",
		GitHubClientID: "x", GitHubClientSecret: "y",
	})
	require.Contains(t, ps, "github")
	require.NotContains(t, ps, "google")
	require.Equal(t, "http://localhost:8080/api/v1/auth/callback?provider=github", ps["github"].RedirectURL())
}
