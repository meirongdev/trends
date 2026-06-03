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
