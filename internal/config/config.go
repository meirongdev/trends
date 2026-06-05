package config

import (
	"os"
	"strconv"
	"strings"
)

// defaultDiscoveryQueries 是 Discovery 的默认 star 区间切片(精简版:门槛 ≥50,宇宙约 5k)。
// 按 star 切片是为了绕开 GitHub Search 单查询最多返回 1000 条的硬上限。
// 用 DISCOVERY_QUERIES(换行或逗号分隔)整体覆盖,可改成按语言/活跃度切片而无需改代码,
// 例如:"language:go stars:>50" 或 "pushed:>2026-01-01 stars:100..500"。
var defaultDiscoveryQueries = []string{
	"stars:50..100",
	"stars:100..250",
	"stars:250..1000",
	"stars:1000..5000",
	"stars:5000..20000",
	"stars:>20000",
}

type Config struct {
	DBPath           string
	GitHubTokens     []string
	GitHubAPIBaseURL string
	GitHubGraphQLURL string
	APIListenAddr    string
	DiscoveryCron    string
	SnapshotCron     string

	DiscoveryQueries  []string
	DiscoveryMaxPages int

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

		DiscoveryQueries:  queriesOrDefault(os.Getenv("DISCOVERY_QUERIES"), defaultDiscoveryQueries),
		DiscoveryMaxPages: intOrDefault(os.Getenv("DISCOVERY_MAX_PAGES"), 10),

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

// splitLines 按换行或逗号分隔,去空白、丢空项。查询本身可含空格
// (如 "language:go stars:>50"),故只在换行/逗号处切。
func splitLines(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func queriesOrDefault(s string, def []string) []string {
	if q := splitLines(s); len(q) > 0 {
		return q
	}
	return def
}

func intOrDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
		return n
	}
	return def
}
