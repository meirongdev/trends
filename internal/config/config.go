package config

import (
	"os"
	"strings"
)

type Config struct {
	DBPath           string
	GitHubTokens     []string
	GitHubAPIBaseURL string
	GitHubGraphQLURL string
	APIListenAddr    string
	DiscoveryCron    string
	SnapshotCron     string

	OAuthBaseURL            string
	GitHubOAuthClientID     string
	GitHubOAuthClientSecret string
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
}

func Load() Config {
	return Config{
		DBPath:           getenv("DB_PATH", "trends.db"),
		GitHubTokens:     splitNonEmpty(os.Getenv("GITHUB_TOKENS")),
		GitHubAPIBaseURL: getenv("GITHUB_API_BASE_URL", "https://api.github.com"),
		GitHubGraphQLURL: getenv("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"),
		APIListenAddr:    getenv("API_LISTEN_ADDR", ":8080"),
		DiscoveryCron:    getenv("DISCOVERY_CRON", "0 1 * * *"),
		SnapshotCron:     getenv("SNAPSHOT_CRON", "0 0 * * *"),

		OAuthBaseURL:            getenv("OAUTH_BASE_URL", "http://localhost:8080"),
		GitHubOAuthClientID:     os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubOAuthClientSecret: os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		GoogleOAuthClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
