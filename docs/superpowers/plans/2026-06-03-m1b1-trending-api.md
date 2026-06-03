# M1b-1 趋势读取 API 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 M1a 物化好的 `trending_rankings` 通过只读 REST API 暴露出来:`/healthz`、`GET /api/v1/trending`(周期/语言/日期/分页)、`GET /api/v1/languages`,并把 HTTP 服务接进单一二进制(与调度器并存,优雅停机)。

**Architecture:** 新增只读查询层方法到 `internal/store`(JOIN `trending_rankings`+`repositories`,语言计数,健康信息);新增 `internal/api` 包(Go 1.22+ `net/http.ServeMux` 方法+路径路由、JSON 辅助、handler 与 DTO);`cmd/trends` 在后台 goroutine 起 `http.Server`(监听 `cfg.APIListenAddr`),与 cron 调度器一起运行,收到信号时优雅关停二者。

**Tech Stack:** Go 1.26 标准库(`net/http`、`encoding/json`、`net/http/httptest`)· `modernc.org/sqlite` · `github.com/stretchr/testify`。无新增第三方依赖。

> **依赖:** M0 + M1a 已在 `main`。`store.DB` 有 `Open`/`SQL()`/`UpsertRepository`/`UpdateRepositoryMetrics`/`ReplaceRankings`、类型 `Repository`(20 字段)、`Ranking`。`repositories(id, github_id, node_id, full_name, owner, name, description, language, homepage, html_url, owner_avatar, stars, forks, open_issues, watchers, is_archived, is_active, repo_created_at, first_seen_at, last_synced_at)` 与 `trending_rankings(period, period_date, repository_id, rank, score, star_delta, language)` 已存在。`config.Config.APIListenAddr` 默认 `:8080`。测试助手 `newTestDB(t)`(store_test.go)、`sampleRepo()`(repository_test.go,github_id 111,Language "Go")。
> **离线构建:** 依赖已缓存。用 `GOPROXY=off go build/test`;不要 `go get`。
> **范围:** 仅 trending/languages/health 的读取 + 服务接线。仓库详情(`/repositories/*`)与搜索(`/search`)属 M1b-2。SEO 的 sitemap/robots 属 M2。

---

## File Structure

| 文件 | 职责 |
|---|---|
| `internal/store/query.go` | 只读查询:`RankedRepository`、`LatestRankingDate`、`ListRankings`、`CountRankings`、`LanguageCount`/`LanguageCounts`、`Health`/`HealthInfo` |
| `internal/store/query_test.go` | 查询层测试(临时 DB) |
| `internal/api/server.go` | `Server`、`NewServer`、`Routes()`、`writeJSON`/`writeError`/`atoiDefault` 辅助 |
| `internal/api/health.go` | `/healthz` handler |
| `internal/api/health_test.go` | health 测试 |
| `internal/api/trending.go` | `/api/v1/trending` handler + DTO |
| `internal/api/trending_test.go` | trending 测试 |
| `internal/api/languages.go` | `/api/v1/languages` handler + DTO |
| `internal/api/languages_test.go` | languages 测试 |
| `cmd/trends/main.go`(改) | 起 HTTP 服务 + 与调度器并存 + 优雅停机 |

---

## Task 1: store 趋势读取(JOIN 榜单 + 仓库)

**Files:**
- Create: `internal/store/query.go`
- Test: `internal/store/query_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/store/query_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// seedRanked 建仓库、设其语言+stars,并写一条 daily 榜单。
func seedRanked(t *testing.T, db *DB, githubID int64, node, fullName, lang string, stars, rank, delta int, score float64, date string) int64 {
	t.Helper()
	id, err := db.UpsertRepository(Repository{
		GitHubID: githubID, NodeID: node, FullName: fullName, Owner: "a", Name: node, HTMLURL: "https://gh/" + fullName, Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(githubID, stars, 0, 0, 0, date+"T00:00:00Z"))
	require.NoError(t, db.ReplaceRankings("daily", date, append(rankingsFor(t, db, "daily", date), Ranking{
		RepositoryID: id, Rank: rank, Score: score, StarDelta: delta, Language: lang,
	})))
	return id
}

// rankingsFor 读回某 period+date 现有榜单(便于增量追加),测试辅助。
func rankingsFor(t *testing.T, db *DB, period, date string) []Ranking {
	t.Helper()
	rows, err := db.SQL().Query(`SELECT repository_id, rank, score, star_delta, COALESCE(language,'') FROM trending_rankings WHERE period=? AND period_date=? ORDER BY rank`, period, date)
	require.NoError(t, err)
	defer rows.Close()
	var out []Ranking
	for rows.Next() {
		var r Ranking
		require.NoError(t, rows.Scan(&r.RepositoryID, &r.Rank, &r.Score, &r.StarDelta, &r.Language))
		out = append(out, r)
	}
	return out
}

func TestLatestRankingDate(t *testing.T) {
	db := newTestDB(t)
	_, ok, err := db.LatestRankingDate("daily")
	require.NoError(t, err)
	require.False(t, ok) // 无榜单

	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: id, Rank: 1, Score: 1, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 1, Score: 1, StarDelta: 20, Language: "Go"}}))

	date, ok, err := db.LatestRankingDate("daily")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "2026-06-10", date)
}

func TestListAndCountRankingsWithLanguageFilterAndPaging(t *testing.T) {
	db := newTestDB(t)
	idGo := seedRanked(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-10")
	_ = seedRanked(t, db, 2, "R2", "a/go2", "Go", 800, 2, 100, 0.5, "2026-06-10")
	_ = seedRanked(t, db, 3, "R3", "a/rust1", "Rust", 500, 3, 50, 0.3, "2026-06-10")

	// 无语言过滤:3 条,按 rank 升序
	total, err := db.CountRankings("daily", "2026-06-10", "")
	require.NoError(t, err)
	require.Equal(t, 3, total)
	all, err := db.ListRankings("daily", "2026-06-10", "", 25, 0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, 1, all[0].Rank)
	require.Equal(t, idGo, all[0].Repo.ID)
	require.Equal(t, "a/go1", all[0].Repo.FullName)
	require.Equal(t, 200, all[0].StarDelta)

	// 语言过滤 Go:2 条
	goTotal, err := db.CountRankings("daily", "2026-06-10", "Go")
	require.NoError(t, err)
	require.Equal(t, 2, goTotal)
	goRows, err := db.ListRankings("daily", "2026-06-10", "Go", 25, 0)
	require.NoError(t, err)
	require.Len(t, goRows, 2)

	// 分页:per_page=1, offset=1 → 第 2 条(rank 2)
	page2, err := db.ListRankings("daily", "2026-06-10", "", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, 2, page2[0].Rank)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run "TestLatestRankingDate|TestListAndCountRankings" -v`
Expected: FAIL — `undefined: RankedRepository`/`LatestRankingDate`/`ListRankings`/`CountRankings`。

- [ ] **Step 3: 实现查询**

`internal/store/query.go`:
```go
package store

import "database/sql"

// RankedRepository 是榜单项 + 仓库展示信息(JOIN 结果)。
type RankedRepository struct {
	Rank      int
	Score     float64
	StarDelta int
	Repo      Repository
}

// repoColsR 是带 r. 前缀、可用于 JOIN 的仓库列清单,顺序与 scanInto 一致。
const repoColsR = `r.id, r.github_id, r.node_id, r.full_name, r.owner, r.name,
       COALESCE(r.description,''), COALESCE(r.language,''), COALESCE(r.homepage,''),
       r.html_url, COALESCE(r.owner_avatar,''), r.stars, r.forks, r.open_issues, r.watchers,
       r.is_archived, r.is_active, COALESCE(r.repo_created_at,''), r.first_seen_at, COALESCE(r.last_synced_at,'')`

// LatestRankingDate 返回某 period 已物化的最新 period_date;无榜单时 ok=false。
func (d *DB) LatestRankingDate(period string) (string, bool, error) {
	var date sql.NullString
	if err := d.db.QueryRow(`SELECT MAX(period_date) FROM trending_rankings WHERE period=?`, period).Scan(&date); err != nil {
		return "", false, err
	}
	if !date.Valid {
		return "", false, nil
	}
	return date.String, true, nil
}

// CountRankings 返回某 period+date(可选语言过滤)的榜单条数。
func (d *DB) CountRankings(period, date, language string) (int, error) {
	q := `SELECT COUNT(*) FROM trending_rankings tr JOIN repositories r ON r.id=tr.repository_id WHERE tr.period=? AND tr.period_date=?`
	args := []any{period, date}
	if language != "" {
		q += ` AND r.language=?`
		args = append(args, language)
	}
	var n int
	err := d.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// ListRankings 返回某 period+date(可选语言过滤)的榜单项 + 仓库信息,按 rank 升序分页。
func (d *DB) ListRankings(period, date, language string, limit, offset int) ([]RankedRepository, error) {
	q := `SELECT tr.rank, tr.score, tr.star_delta, ` + repoColsR + `
FROM trending_rankings tr JOIN repositories r ON r.id = tr.repository_id
WHERE tr.period=? AND tr.period_date=?`
	args := []any{period, date}
	if language != "" {
		q += ` AND r.language=?`
		args = append(args, language)
	}
	q += ` ORDER BY tr.rank LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RankedRepository
	for rows.Next() {
		var rr RankedRepository
		var archived, active int
		if err := rows.Scan(
			&rr.Rank, &rr.Score, &rr.StarDelta,
			&rr.Repo.ID, &rr.Repo.GitHubID, &rr.Repo.NodeID, &rr.Repo.FullName, &rr.Repo.Owner, &rr.Repo.Name,
			&rr.Repo.Description, &rr.Repo.Language, &rr.Repo.Homepage, &rr.Repo.HTMLURL, &rr.Repo.OwnerAvatar,
			&rr.Repo.Stars, &rr.Repo.Forks, &rr.Repo.OpenIssues, &rr.Repo.Watchers,
			&archived, &active, &rr.Repo.RepoCreatedAt, &rr.Repo.FirstSeenAt, &rr.Repo.LastSyncedAt,
		); err != nil {
			return nil, err
		}
		rr.Repo.IsArchived = archived == 1
		rr.Repo.IsActive = active == 1
		out = append(out, rr)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/query.go internal/store/query_test.go
git commit -m "$(printf 'feat(store): read queries for trending rankings (join, count, latest date)\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 2: store 语言计数 + 健康信息

**Files:**
- Modify: `internal/store/query.go`
- Modify: `internal/store/query_test.go`

- [ ] **Step 1: 追加失败的测试**

在 `internal/store/query_test.go` 末尾追加:
```go
func TestLanguageCounts(t *testing.T) {
	db := newTestDB(t)
	mk := func(gid int64, node, full, lang string) {
		_, err := db.UpsertRepository(Repository{GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, HTMLURL: "u", Language: lang})
		require.NoError(t, err)
	}
	mk(1, "R1", "a/1", "Go")
	mk(2, "R2", "a/2", "Go")
	mk(3, "R3", "a/3", "Rust")
	mk(4, "R4", "a/4", "") // 空语言不计入

	counts, err := db.LanguageCounts()
	require.NoError(t, err)
	require.Equal(t, []LanguageCount{{Language: "Go", Count: 2}, {Language: "Rust", Count: 1}}, counts)
}

func TestHealthInfo(t *testing.T) {
	db := newTestDB(t)
	h, err := db.HealthInfo()
	require.NoError(t, err)
	require.Equal(t, 0, h.ActiveRepos)
	require.Equal(t, "", h.LastSyncedAt)

	_, err = db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(111, 100, 0, 0, 0, "2026-06-10T00:00:00Z"))

	h, err = db.HealthInfo()
	require.NoError(t, err)
	require.Equal(t, 1, h.ActiveRepos)
	require.Equal(t, "2026-06-10T00:00:00Z", h.LastSyncedAt)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run "TestLanguageCounts|TestHealthInfo" -v`
Expected: FAIL — `undefined: LanguageCount`/`LanguageCounts`/`Health`/`HealthInfo`。

- [ ] **Step 3: 实现**

在 `internal/store/query.go` 末尾追加:
```go
// LanguageCount 是某语言下的活跃仓库数。
type LanguageCount struct {
	Language string
	Count    int
}

// LanguageCounts 返回各语言的活跃仓库数,按数量降序、同数按语言名升序;排除空语言。
func (d *DB) LanguageCounts() ([]LanguageCount, error) {
	rows, err := d.db.Query(`
SELECT language, COUNT(*) FROM repositories
WHERE is_active=1 AND language IS NOT NULL AND language <> ''
GROUP BY language
ORDER BY COUNT(*) DESC, language`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LanguageCount
	for rows.Next() {
		var lc LanguageCount
		if err := rows.Scan(&lc.Language, &lc.Count); err != nil {
			return nil, err
		}
		out = append(out, lc)
	}
	return out, rows.Err()
}

// Health 是 /healthz 暴露的运行信息。
type Health struct {
	LastSyncedAt string
	ActiveRepos  int
}

// HealthInfo 返回最近一次采集同步时间与活跃仓库数(无数据时分别为 ""/0)。
func (d *DB) HealthInfo() (Health, error) {
	var h Health
	var last sql.NullString
	if err := d.db.QueryRow(`SELECT MAX(last_synced_at) FROM repositories`).Scan(&last); err != nil {
		return h, err
	}
	h.LastSyncedAt = last.String
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM repositories WHERE is_active=1`).Scan(&h.ActiveRepos); err != nil {
		return h, err
	}
	return h, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/query.go internal/store/query_test.go
git commit -m "$(printf 'feat(store): language counts and health info queries\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 3: api 包骨架 + /healthz

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/health.go`
- Test: `internal/api/health_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/api/health_test.go`:
```go
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/api/...`
Expected: FAIL — `undefined: Server`/`NewServer`。

- [ ] **Step 3: 实现骨架 + health**

`internal/api/server.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/meirongdev/trends/internal/store"
)

// Server 持有只读存储句柄,提供 HTTP 路由。
type Server struct {
	db *store.DB
}

func NewServer(db *store.DB) *Server {
	return &Server{db: db}
}

// Routes 构建并返回 HTTP 路由(Go 1.22+ 方法+路径模式)。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/v1/trending", s.handleTrending)
	mux.HandleFunc("GET /api/v1/languages", s.handleLanguages)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// atoiDefault 解析十进制整数,失败或空串返回 def。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
```

`internal/api/health.go`:
```go
package api

import "net/http"

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	h, err := s.db.HealthInfo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"last_synced_at": h.LastSyncedAt,
		"active_repos":   h.ActiveRepos,
	})
}
```

> 注:`Routes()` 同时注册了 `handleTrending`/`handleLanguages`,它们在 Task 4/5 实现。本任务为让 `internal/api` 包能编译,需在 `server.go` 之外**先放占位**——见下方 Step 3b。

- [ ] **Step 3b: 放置 trending/languages 占位以便编译**

为了本任务可编译(Task 4/5 会替换),创建最小占位文件 `internal/api/trending.go`:
```go
package api

import "net/http"

func (s *Server) handleTrending(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
```
和 `internal/api/languages.go`:
```go
package api

import "net/http"

func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/api/...`
Expected: PASS（`TestHealthzReportsStatus`）。

- [ ] **Step 5: 提交**

```bash
git add internal/api/server.go internal/api/health.go internal/api/health_test.go internal/api/trending.go internal/api/languages.go
git commit -m "$(printf 'feat(api): server skeleton, json helpers, and /healthz\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 4: GET /api/v1/trending

**Files:**
- Modify: `internal/api/trending.go`(替换占位)
- Test: `internal/api/trending_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/api/trending_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func seedRanking(t *testing.T, db *store.DB, gid int64, node, full, lang string, stars, rank, delta int, score float64, date string) {
	t.Helper()
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, HTMLURL: "https://gh/" + full, Language: lang, Description: "d",
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, 0, 0, 0, date+"T00:00:00Z"))
	existing := readRankings(t, db, "daily", date)
	require.NoError(t, db.ReplaceRankings("daily", date, append(existing, store.Ranking{
		RepositoryID: id, Rank: rank, Score: score, StarDelta: delta, Language: lang,
	})))
}

func readRankings(t *testing.T, db *store.DB, period, date string) []store.Ranking {
	t.Helper()
	rows, err := db.SQL().Query(`SELECT repository_id, rank, score, star_delta, COALESCE(language,'') FROM trending_rankings WHERE period=? AND period_date=? ORDER BY rank`, period, date)
	require.NoError(t, err)
	defer rows.Close()
	var out []store.Ranking
	for rows.Next() {
		var r store.Ranking
		require.NoError(t, rows.Scan(&r.RepositoryID, &r.Rank, &r.Score, &r.StarDelta, &r.Language))
		out = append(out, r)
	}
	return out
}

type trendingResp struct {
	Period  string `json:"period"`
	Date    string `json:"date"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Total   int    `json:"total"`
	Items   []struct {
		Rank       int     `json:"rank"`
		Score      float64 `json:"score"`
		StarDelta  int     `json:"star_delta"`
		Repository struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			Language string `json:"language"`
			Stars    int    `json:"stars"`
		} `json:"repository"`
	} `json:"items"`
}

func TestTrendingDefaultsToLatestDate(t *testing.T) {
	s, db := newTestServer(t)
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-09")
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1200, 1, 250, 0.95, "2026-06-10") // 更新到 06-10

	rec := doGET(t, s, "/api/v1/trending")
	require.Equal(t, http.StatusOK, rec.Code)
	var body trendingResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "daily", body.Period)
	require.Equal(t, "2026-06-10", body.Date) // 默认取最新
	require.Equal(t, 1, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, 1, body.Items[0].Rank)
	require.Equal(t, "a/go1", body.Items[0].Repository.FullName)
	require.Equal(t, 250, body.Items[0].StarDelta)
}

func TestTrendingLanguageFilterAndPaging(t *testing.T) {
	s, db := newTestServer(t)
	seedRanking(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-10")
	seedRanking(t, db, 2, "R2", "a/go2", "Go", 800, 2, 100, 0.5, "2026-06-10")
	seedRanking(t, db, 3, "R3", "a/rust1", "Rust", 500, 3, 50, 0.3, "2026-06-10")

	rec := doGET(t, s, "/api/v1/trending?language=Go&date=2026-06-10")
	var body trendingResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.Total)
	require.Len(t, body.Items, 2)

	rec = doGET(t, s, "/api/v1/trending?date=2026-06-10&per_page=1&page=2")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 3, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, 2, body.Items[0].Rank) // 第 2 页第 1 条
}

func TestTrendingRejectsBadPeriod(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/trending?period=hourly")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTrendingEmptyWhenNoRankings(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/trending")
	require.Equal(t, http.StatusOK, rec.Code)
	var body trendingResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Total)
	require.NotNil(t, body.Items)
	require.Len(t, body.Items, 0)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run TestTrending -v`
Expected: FAIL — 占位返回 501,断言不符。

- [ ] **Step 3: 实现 handler**

把 `internal/api/trending.go` 整体替换为:
```go
package api

import "net/http"

type repositoryDTO struct {
	ID          int64  `json:"id"`
	FullName    string `json:"full_name"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	HTMLURL     string `json:"html_url"`
	OwnerAvatar string `json:"owner_avatar"`
}

type trendingItemDTO struct {
	Rank       int           `json:"rank"`
	Score      float64       `json:"score"`
	StarDelta  int           `json:"star_delta"`
	Repository repositoryDTO `json:"repository"`
}

type trendingResponseDTO struct {
	Period  string            `json:"period"`
	Date    string            `json:"date"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Total   int               `json:"total"`
	Items   []trendingItemDTO `json:"items"`
}

var validPeriods = map[string]bool{"daily": true, "weekly": true, "monthly": true}

func (s *Server) handleTrending(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "daily"
	}
	if !validPeriods[period] {
		writeError(w, http.StatusBadRequest, "invalid period (use daily|weekly|monthly)")
		return
	}
	language := q.Get("language")
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage := atoiDefault(q.Get("per_page"), 25)
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	date := q.Get("date")
	if date == "" {
		latest, ok, err := s.db.LatestRankingDate(period)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
		if !ok {
			writeJSON(w, http.StatusOK, trendingResponseDTO{
				Period: period, Page: page, PerPage: perPage, Total: 0, Items: []trendingItemDTO{},
			})
			return
		}
		date = latest
	}

	total, err := s.db.CountRankings(period, date, language)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	ranked, err := s.db.ListRankings(period, date, language, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	items := make([]trendingItemDTO, 0, len(ranked))
	for _, rr := range ranked {
		items = append(items, trendingItemDTO{
			Rank: rr.Rank, Score: rr.Score, StarDelta: rr.StarDelta,
			Repository: repositoryDTO{
				ID: rr.Repo.ID, FullName: rr.Repo.FullName, Owner: rr.Repo.Owner, Name: rr.Repo.Name,
				Description: rr.Repo.Description, Language: rr.Repo.Language, Stars: rr.Repo.Stars,
				HTMLURL: rr.Repo.HTMLURL, OwnerAvatar: rr.Repo.OwnerAvatar,
			},
		})
	}
	writeJSON(w, http.StatusOK, trendingResponseDTO{
		Period: period, Date: date, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/api/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/api/trending.go internal/api/trending_test.go
git commit -m "$(printf 'feat(api): GET /api/v1/trending with period/language/date/pagination\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 5: GET /api/v1/languages

**Files:**
- Modify: `internal/api/languages.go`(替换占位)
- Test: `internal/api/languages_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/api/languages_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestLanguagesReturnsCounts(t *testing.T) {
	s, db := newTestServer(t)
	mk := func(gid int64, node, full, lang string) {
		_, err := db.UpsertRepository(store.Repository{GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, HTMLURL: "u", Language: lang})
		require.NoError(t, err)
	}
	mk(1, "R1", "a/1", "Go")
	mk(2, "R2", "a/2", "Go")
	mk(3, "R3", "a/3", "Rust")

	rec := doGET(t, s, "/api/v1/languages")
	require.Equal(t, http.StatusOK, rec.Code)
	var body []struct {
		Language string `json:"language"`
		Count    int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, "Go", body[0].Language)
	require.Equal(t, 2, body[0].Count)
	require.Equal(t, "Rust", body[1].Language)
	require.Equal(t, 1, body[1].Count)
}

func TestLanguagesEmptyReturnsEmptyArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/languages")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[]\n", rec.Body.String()) // 空数组而非 null
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run TestLanguages -v`
Expected: FAIL — 占位返回 501。

- [ ] **Step 3: 实现 handler**

把 `internal/api/languages.go` 整体替换为:
```go
package api

import "net/http"

type languageDTO struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
}

func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	langs, err := s.db.LanguageCounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	out := make([]languageDTO, 0, len(langs))
	for _, l := range langs {
		out = append(out, languageDTO{Language: l.Language, Count: l.Count})
	}
	writeJSON(w, http.StatusOK, out)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/api/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/api/languages.go internal/api/languages_test.go
git commit -m "$(printf 'feat(api): GET /api/v1/languages\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 6: 把 HTTP 服务接进 main(与调度器并存)

**Files:**
- Modify: `cmd/trends/main.go`

- [ ] **Step 1: 改写 main.go**

把 `cmd/trends/main.go` 整体替换为下面内容。变化:`import` 增加 `"net/http"` 与 `internal/api`;在调度器之外、阻塞等待信号之前,起一个后台 `http.Server`(监听 `cfg.APIListenAddr`,挂 `api.NewServer(db).Routes()`);收到信号时优雅关停 HTTP(`Shutdown`)与调度器(`Stop`)。`RUN_ONCE` 分支不起服务。

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meirongdev/trends/internal/api"
	"github.com/meirongdev/trends/internal/config"
	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/ingest"
	"github.com/meirongdev/trends/internal/scheduler"
	"github.com/meirongdev/trends/internal/scoring"
	"github.com/meirongdev/trends/internal/store"
)

// MVP 阶段的发现查询:按 star 区间切片,后续可迁入配置。
var discoveryQueries = []string{
	"stars:50..100",
	"stars:100..250",
	"stars:250..1000",
	"stars:1000..5000",
	"stars:>5000",
}

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	gh := github.NewClient(cfg.GitHubAPIBaseURL, cfg.GitHubGraphQLURL, cfg.GitHubTokens)
	scoreCfg := scoring.DefaultConfig()

	runDiscovery := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_, err := ingest.RunDiscovery(ctx, db, gh, discoveryQueries, 10)
		return err
	}
	runSnapshot := func(date string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		return ingest.RunSnapshot(ctx, db, gh, date, 100)
	}
	runScoring := func(date string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return ingest.RunScoring(ctx, db, date, scoreCfg)
	}

	// RUN_ONCE=discovery|snapshot|score 用于手动触发一次后退出;失败以非零码退出。不起 HTTP 服务。
	switch os.Getenv("RUN_ONCE") {
	case "discovery":
		if err := runDiscovery(); err != nil {
			slog.Error("discovery run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "snapshot":
		if err := runSnapshot(todayUTC()); err != nil {
			slog.Error("snapshot run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "score":
		if err := runScoring(todayUTC()); err != nil {
			slog.Error("score run-once failed", "err", err)
			os.Exit(1)
		}
		return
	}

	sch, err := scheduler.New(
		scheduler.Job{Spec: cfg.DiscoveryCron, Run: func() {
			if err := runDiscovery(); err != nil {
				slog.Error("discovery job", "err", err)
			}
		}},
		// 快照成功后链式评分;两步共用同一 as-of 日期,确保榜单与刚写入的快照对齐
		// (避免快照跨过 UTC 午夜时,评分用到与快照不同的日期)。
		scheduler.Job{Spec: cfg.SnapshotCron, Run: func() {
			date := todayUTC()
			if err := runSnapshot(date); err != nil {
				slog.Error("snapshot job", "err", err)
				return
			}
			if err := runScoring(date); err != nil {
				slog.Error("scoring job", "err", err)
			}
		}},
	)
	if err != nil {
		slog.Error("scheduler", "err", err)
		os.Exit(1)
	}
	sch.Start()
	defer sch.Stop()

	httpServer := &http.Server{
		Addr:              cfg.APIListenAddr,
		Handler:           api.NewServer(db).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("api listening", "addr", cfg.APIListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
		}
	}()

	slog.Info("trends started", "discovery_cron", cfg.DiscoveryCron, "snapshot_cron", cfg.SnapshotCron)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "err", err)
	}
}
```

- [ ] **Step 2: 编译 + 全量测试 + 冒烟**

Run:
```bash
GOPROXY=off go vet ./...
GOPROXY=off go build ./...
GOPROXY=off go test ./...
# 冒烟:起服务、curl /healthz、关停。空 DB 上应 200。
rm -f /tmp/trends_api.db*
DB_PATH=/tmp/trends_api.db API_LISTEN_ADDR=127.0.0.1:18080 GOPROXY=off go run ./cmd/trends &
SRV_PID=$!
sleep 2
curl -s -o /dev/null -w "healthz=%{http_code}\n" http://127.0.0.1:18080/healthz
curl -s -o /dev/null -w "trending=%{http_code}\n" http://127.0.0.1:18080/api/v1/trending
curl -s -o /dev/null -w "languages=%{http_code}\n" http://127.0.0.1:18080/api/v1/languages
kill -TERM $SRV_PID 2>/dev/null; wait $SRV_PID 2>/dev/null
rm -f /tmp/trends_api.db*
```
Expected: vet/build/test 全过;三个 curl 都打印 `=200`。

> 说明:`go run` 后台进程 + `sleep 2` 让服务起来;`curl` 需要本机网络回环(127.0.0.1)。若沙箱禁本地回环导致 curl 失败,改用 `go test` 已覆盖的 handler 测试作为验证依据,并在报告里说明 curl 被环境阻断。

- [ ] **Step 3: 提交**

```bash
git add cmd/trends/main.go
git commit -m "$(printf 'feat: serve read API alongside scheduler with graceful shutdown\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Self-Review(已核对)

- **Spec 覆盖**:对应 spec §7 的 `/api/v1/trending`(period/language/date/分页,默认取最新日期)、`/api/v1/languages`、`/healthz`;§10 性能上加了 `Content-Type`/JSON、`ReadHeaderTimeout`(缓存头与 CDN 留待 M2);§3 单二进制内 API 与调度器并存 + 优雅停机。**仓库详情 `/repositories/*`、搜索 `/search`、sitemap/robots 属 M1b-2 / M2,不在本计划。**
- **占位符**:无 TBD/TODO;每个代码步骤均有完整代码与命令。Task 3 刻意先放 trending/languages 的 501 占位以保证包可编译,Task 4/5 整体替换——已在步骤中写明完整替换内容。
- **类型一致性**:`store.RankedRepository{Rank,Score,StarDelta,Repo Repository}`、`LatestRankingDate/ListRankings/CountRankings/LanguageCounts/HealthInfo`、`LanguageCount{Language,Count}`、`Health{LastSyncedAt,ActiveRepos}`;api 侧 `Server`/`NewServer`/`Routes`/`writeJSON`/`writeError`/`atoiDefault`、DTO 字段在各任务间一致。`repoColsR` 列顺序与 `scanInto`/`ListRankings` 的扫描顺序一致(20 列)。
- **JOIN 歧义**:`language` 在 `repositories` 与 `trending_rankings` 都有,故 JOIN 查询一律用 `r.` / `tr.` 限定,`repoColsR` 带 `r.` 前缀。
- **空集**:`/trending` 无榜单时返回空 `items` 数组(非 null);`/languages` 用 `make([]T,0,...)` 保证空数组序列化为 `[]`。

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-03-m1b1-trending-api.md`.
