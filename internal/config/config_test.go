package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_PATH", "")
	t.Setenv("GITHUB_TOKENS", "")
	cfg := Load()
	require.Equal(t, "trends.db", cfg.DBPath)
	require.Equal(t, "https://api.github.com", cfg.GitHubAPIBaseURL)
	require.Equal(t, "https://api.github.com/graphql", cfg.GitHubGraphQLURL)
	require.Empty(t, cfg.GitHubTokens)
}

func TestLoadParsesTokens(t *testing.T) {
	t.Setenv("DB_PATH", "/tmp/x.db")
	t.Setenv("GITHUB_TOKENS", " tok1 , tok2 ,")
	cfg := Load()
	require.Equal(t, "/tmp/x.db", cfg.DBPath)
	require.Equal(t, []string{"tok1", "tok2"}, cfg.GitHubTokens)
}

func TestLoadOAuth(t *testing.T) {
	t.Setenv("OAUTH_BASE_URL", "https://trends.example")
	t.Setenv("GITHUB_OAUTH_CLIENT_ID", "gid")
	t.Setenv("GITHUB_OAUTH_CLIENT_SECRET", "gsec")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "ggid")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "ggsec")

	c := Load()
	require.Equal(t, "https://trends.example", c.OAuthBaseURL)
	require.Equal(t, "gid", c.GitHubOAuthClientID)
	require.Equal(t, "gsec", c.GitHubOAuthClientSecret)
	require.Equal(t, "ggid", c.GoogleOAuthClientID)
	require.Equal(t, "ggsec", c.GoogleOAuthClientSecret)
}

func TestLoadOAuthDefaultBaseURL(t *testing.T) {
	c := Load()
	require.Equal(t, "http://localhost:8080", c.OAuthBaseURL)
}
