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

func TestLoadDiscoveryDefaults(t *testing.T) {
	t.Setenv("DISCOVERY_QUERIES", "")
	t.Setenv("DISCOVERY_MAX_PAGES", "")
	c := Load()
	require.Equal(t, defaultDiscoveryQueries, c.DiscoveryQueries)
	require.Equal(t, 10, c.DiscoveryMaxPages)
}

func TestLoadDiscoveryOverride(t *testing.T) {
	// 换行与逗号混用、首尾空白、空项,以及含空格的查询都应正确解析。
	t.Setenv("DISCOVERY_QUERIES", "stars:>1000\nstars:500..1000, language:go stars:50..100 ,")
	t.Setenv("DISCOVERY_MAX_PAGES", "5")
	c := Load()
	require.Equal(t, []string{"stars:>1000", "stars:500..1000", "language:go stars:50..100"}, c.DiscoveryQueries)
	require.Equal(t, 5, c.DiscoveryMaxPages)
}

func TestLoadDiscoveryMaxPagesInvalidFallsBack(t *testing.T) {
	t.Setenv("DISCOVERY_MAX_PAGES", "abc")
	require.Equal(t, 10, Load().DiscoveryMaxPages)
	t.Setenv("DISCOVERY_MAX_PAGES", "0")
	require.Equal(t, 10, Load().DiscoveryMaxPages)
}
