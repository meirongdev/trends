# M1b-2 仓库详情 + 搜索 API 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补全只读 API:`GET /api/v1/repositories/{id}`(详情 + 最佳日榜名次)、`/repositories/{id}/snapshots`(star 历史曲线数据)、`/repositories/{id}/rankings`(历史上榜记录)、`GET /api/v1/search`(按名称/简介全文搜索,可按语言过滤、分页)。

**Architecture:** `internal/store` 增加只读查询:按内部 id 取仓库、最佳日榜名次、快照序列、上榜历史、文本搜索 + 计数。`internal/api` 增加对应 handler(用 Go 1.22+ `ServeMux` 的 `{id}` 路径通配 + `r.PathValue`),复用现有 `repositoryDTO`/`writeJSON`/`writeError`/`atoiDefault`,并把新路由注册进 `Routes()`。

**Tech Stack:** Go 1.26 标准库 · `modernc.org/sqlite` · `github.com/stretchr/testify`。无新增第三方依赖。

> **依赖:** M0/M1a/M1b-1 已在 `main`。已有:`store.Repository`(20 字段)、`store.Snapshot{RepositoryID,Date,Stars,Forks,OpenIssues,Watchers,StarDelta}`、`const repoSelectColumns`(无前缀列清单,在 repository.go)、`scanRepo(*sql.Row)`/`scanRepoRows(*sql.Rows)`(repository.go)、`store.DB` 已有 `GetRepositoryByGitHubID`。`internal/api` 有 `Server`/`Routes()`/`writeJSON`/`writeError`/`atoiDefault`/`validPeriods`、`repositoryDTO`(id/full_name/owner/name/description/language/stars/html_url/owner_avatar)、测试助手 `newTestServer(t)`/`doGET(t,s,target)`(health_test.go)与 `seedRanking(...)`/`readRankings(...)`(trending_test.go)。`internal/store` 测试助手 `newTestDB(t)`/`sampleRepo()`/`seedRanked(...)`(query_test.go)。
> **离线构建:** 依赖已缓存。用 `GOPROXY=off go build/test`;不要 `go get`。
> **范围:** 仅本 4 类端点。徽章/开发者/话题/SEO 属后续阶段。

---

## File Structure

| 文件 | 职责 |
|---|---|
| `internal/store/repo_detail.go` | `GetRepositoryByID`、`BestDailyRank`、`RepositorySnapshots`、`RankingHistory`/`RepositoryRankingHistory` |
| `internal/store/repo_detail_test.go` | 上述查询测试 |
| `internal/store/search.go` | `SearchRepositoriesByText`、`SearchRepositoriesCount` |
| `internal/store/search_test.go` | 搜索查询测试 |
| `internal/api/repositories.go` | `/repositories/{id}` + `/snapshots` + `/rankings` handler 与 DTO |
| `internal/api/repositories_test.go` | 上述 handler 测试 |
| `internal/api/search.go` | `/search` handler 与 DTO |
| `internal/api/search_test.go` | 搜索 handler 测试 |
| `internal/api/server.go`(改) | 注册新路由 |

---

## Task 1: store — 按 id 取仓库 + 最佳日榜名次

**Files:**
- Create: `internal/store/repo_detail.go`
- Test: `internal/store/repo_detail_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/store/repo_detail_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetRepositoryByID(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	got, ok, err := db.GetRepositoryByID(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, id, got.ID)
	require.Equal(t, "octocat/hello", got.FullName)

	_, ok, err = db.GetRepositoryByID(99999)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestBestDailyRank(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	_, ok, err := db.BestDailyRank(id)
	require.NoError(t, err)
	require.False(t, ok) // 没上过榜

	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: id, Rank: 5, Score: 0.5, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 2, Score: 0.8, StarDelta: 20, Language: "Go"}}))
	// weekly 名次更高也不应影响 daily 最佳
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 30, Language: "Go"}}))

	best, ok, err := db.BestDailyRank(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2, best) // daily 最小 rank
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run "TestGetRepositoryByID|TestBestDailyRank" -v`
Expected: FAIL — `db.GetRepositoryByID`/`db.BestDailyRank` undefined。

- [ ] **Step 3: 实现**

`internal/store/repo_detail.go`:
```go
package store

import (
	"database/sql"
	"errors"
)

// GetRepositoryByID 按内部 id 取仓库;不存在时 ok=false。
func (d *DB) GetRepositoryByID(id int64) (Repository, bool, error) {
	r, err := scanRepo(d.db.QueryRow(`SELECT `+repoSelectColumns+` FROM repositories WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, false, nil
	}
	if err != nil {
		return Repository{}, false, err
	}
	return r, true, nil
}

// BestDailyRank 返回该仓库在 daily 榜上拿到过的最佳(最小)名次;从未上榜时 ok=false。
func (d *DB) BestDailyRank(repoID int64) (int, bool, error) {
	var rank sql.NullInt64
	if err := d.db.QueryRow(
		`SELECT MIN(rank) FROM trending_rankings WHERE repository_id=? AND period='daily'`,
		repoID).Scan(&rank); err != nil {
		return 0, false, err
	}
	if !rank.Valid {
		return 0, false, nil
	}
	return int(rank.Int64), true, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/repo_detail.go internal/store/repo_detail_test.go
git commit -m "$(printf 'feat(store): GetRepositoryByID and BestDailyRank queries\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 2: store — 快照序列 + 上榜历史

**Files:**
- Modify: `internal/store/repo_detail.go`
- Modify: `internal/store/repo_detail_test.go`

- [ ] **Step 1: 追加失败的测试**

在 `internal/store/repo_detail_test.go` 末尾追加:
```go
func TestRepositorySnapshotsRangeAndOrder(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	for _, s := range []Snapshot{
		{RepositoryID: id, Date: "2026-06-08", Stars: 100, Forks: 5, OpenIssues: 1, Watchers: 3, StarDelta: 10},
		{RepositoryID: id, Date: "2026-06-09", Stars: 130, Forks: 6, OpenIssues: 2, Watchers: 4, StarDelta: 30},
		{RepositoryID: id, Date: "2026-06-10", Stars: 150, Forks: 7, OpenIssues: 2, Watchers: 4, StarDelta: 20},
	} {
		require.NoError(t, db.InsertSnapshot(s))
	}

	// 全量,按日期升序
	all, err := db.RepositorySnapshots(id, "", "")
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, "2026-06-08", all[0].Date)
	require.Equal(t, 100, all[0].Stars)
	require.Equal(t, "2026-06-10", all[2].Date)

	// 区间 [06-09, 06-10]
	rng, err := db.RepositorySnapshots(id, "2026-06-09", "2026-06-10")
	require.NoError(t, err)
	require.Len(t, rng, 2)
	require.Equal(t, "2026-06-09", rng[0].Date)
}

func TestRepositoryRankingHistory(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: id, Rank: 5, Score: 0.5, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 2, Score: 0.8, StarDelta: 20, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 30, Language: "Go"}}))

	hist, err := db.RepositoryRankingHistory(id)
	require.NoError(t, err)
	require.Len(t, hist, 3)
	// 按 period_date 降序;最新日期(06-10)在前
	require.Equal(t, "2026-06-10", hist[0].Date)
	// 同日期内按 period 名次:daily 与 weekly 都在 06-10,确认字段映射
	found := map[string]RankingHistory{}
	for _, h := range hist {
		found[h.Period+"@"+h.Date] = h
	}
	require.Equal(t, 2, found["daily@2026-06-10"].Rank)
	require.Equal(t, 1, found["weekly@2026-06-10"].Rank)
	require.Equal(t, 5, found["daily@2026-06-09"].Rank)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run "TestRepositorySnapshots|TestRepositoryRankingHistory" -v`
Expected: FAIL — `db.RepositorySnapshots`/`db.RepositoryRankingHistory`/`RankingHistory` undefined。

- [ ] **Step 3: 实现**

在 `internal/store/repo_detail.go` 末尾追加:
```go
// RepositorySnapshots 返回某仓库 [from, to] 内的快照,按日期升序;from/to 为空表示不设该端边界。
func (d *DB) RepositorySnapshots(repoID int64, from, to string) ([]Snapshot, error) {
	q := `SELECT repository_id, snapshot_date, stars, forks, open_issues, watchers, star_delta
FROM repository_snapshots WHERE repository_id=?`
	args := []any{repoID}
	if from != "" {
		q += ` AND snapshot_date >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND snapshot_date <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY snapshot_date`

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.RepositoryID, &s.Date, &s.Stars, &s.Forks, &s.OpenIssues, &s.Watchers, &s.StarDelta); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RankingHistory 是某仓库一条历史上榜记录。
type RankingHistory struct {
	Period    string
	Date      string
	Rank      int
	Score     float64
	StarDelta int
}

// RepositoryRankingHistory 返回某仓库所有周期的历史上榜记录,按日期降序、同日按 period。
func (d *DB) RepositoryRankingHistory(repoID int64) ([]RankingHistory, error) {
	rows, err := d.db.Query(`
SELECT period, period_date, rank, score, star_delta
FROM trending_rankings WHERE repository_id=?
ORDER BY period_date DESC, period`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RankingHistory
	for rows.Next() {
		var h RankingHistory
		if err := rows.Scan(&h.Period, &h.Date, &h.Rank, &h.Score, &h.StarDelta); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/repo_detail.go internal/store/repo_detail_test.go
git commit -m "$(printf 'feat(store): repository snapshot series and ranking history queries\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 3: store — 文本搜索 + 计数

**Files:**
- Create: `internal/store/search.go`
- Test: `internal/store/search_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/store/search_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func mkRepo(t *testing.T, db *DB, gid int64, node, full, desc, lang string, stars int) {
	t.Helper()
	_, err := db.UpsertRepository(Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, Description: desc, HTMLURL: "u", Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, 0, 0, 0, "2026-06-10T00:00:00Z"))
}

func TestSearchRepositoriesByTextMatchesNameAndDescription(t *testing.T) {
	db := newTestDB(t)
	mkRepo(t, db, 1, "R1", "vercel/next.js", "The React framework", "JavaScript", 1000)
	mkRepo(t, db, 2, "R2", "facebook/react", "A JavaScript library", "JavaScript", 5000)
	mkRepo(t, db, 3, "R3", "golang/go", "The Go language", "Go", 3000)

	// 名称匹配
	total, err := db.SearchRepositoriesCount("react", "")
	require.NoError(t, err)
	require.Equal(t, 2, total) // "vercel/next.js"(desc 含 React)+ "facebook/react"
	rows, err := db.SearchRepositoriesByText("react", "", 25, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "facebook/react", rows[0].FullName) // 按 stars 降序:5000 在前

	// 语言过滤
	total, err = db.SearchRepositoriesCount("react", "JavaScript")
	require.NoError(t, err)
	require.Equal(t, 2, total)
	total, err = db.SearchRepositoriesCount("language", "Go")
	require.NoError(t, err)
	require.Equal(t, 1, total) // 只有 golang/go 的 desc 含 "language" 且语言 Go

	// 分页
	page2, err := db.SearchRepositoriesByText("react", "", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "vercel/next.js", page2[0].FullName) // stars 次高
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run TestSearchRepositoriesByText -v`
Expected: FAIL — `db.SearchRepositoriesCount`/`db.SearchRepositoriesByText` undefined。

- [ ] **Step 3: 实现**

`internal/store/search.go`:
```go
package store

// searchWhere 构造搜索的 WHERE 子句与参数:在活跃仓库中按 full_name/description 模糊匹配,
// 可选语言过滤。q 作为 LIKE 值绑定(防注入)。
func searchWhere(q, language string) (string, []any) {
	where := ` WHERE is_active=1 AND (full_name LIKE ? OR COALESCE(description,'') LIKE ?)`
	pat := "%" + q + "%"
	args := []any{pat, pat}
	if language != "" {
		where += ` AND language=?`
		args = append(args, language)
	}
	return where, args
}

// SearchRepositoriesCount 返回匹配的活跃仓库总数。
func (d *DB) SearchRepositoriesCount(q, language string) (int, error) {
	where, args := searchWhere(q, language)
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM repositories`+where, args...).Scan(&n)
	return n, err
}

// SearchRepositoriesByText 返回匹配的活跃仓库,按 stars 降序、同 stars 按 full_name,分页。
func (d *DB) SearchRepositoriesByText(q, language string, limit, offset int) ([]Repository, error) {
	where, args := searchWhere(q, language)
	args = append(args, limit, offset)
	rows, err := d.db.Query(`SELECT `+repoSelectColumns+` FROM repositories`+where+` ORDER BY stars DESC, full_name LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Repository
	for rows.Next() {
		r, err := scanRepoRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/search.go internal/store/search_test.go
git commit -m "$(printf 'feat(store): full-text repository search with language filter\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 4: api — GET /api/v1/repositories/{id}

**Files:**
- Create: `internal/api/repositories.go`
- Modify: `internal/api/server.go`(注册路由)
- Test: `internal/api/repositories_test.go`

- [ ] **Step 1: 注册路由**

在 `internal/api/server.go` 的 `Routes()` 里,`languages` 行之后加一行:
```go
	mux.HandleFunc("GET /api/v1/repositories/{id}", s.handleRepository)
```
即把:
```go
	mux.HandleFunc("GET /api/v1/languages", s.handleLanguages)
	return mux
```
替换为:
```go
	mux.HandleFunc("GET /api/v1/languages", s.handleLanguages)
	mux.HandleFunc("GET /api/v1/repositories/{id}", s.handleRepository)
	return mux
```

- [ ] **Step 2: 写失败的测试**

`internal/api/repositories_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// seedRepoWithMetrics 建仓库并设其指标,返回内部 id。
func seedRepoWithMetrics(t *testing.T, db *store.DB, gid int64, node, full, lang string, stars, forks, issues int) int64 {
	t.Helper()
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, Description: "d", HTMLURL: "https://gh/" + full, Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, forks, issues, 0, "2026-06-10T00:00:00Z"))
	return id
}

func TestRepositoryDetailReturnsRepoAndBestRank(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{{RepositoryID: id, Rank: 3, Score: 0.7, StarDelta: 40, Language: "Go"}}))

	rec := doGET(t, s, "/api/v1/repositories/1") // 注意:路径用内部 id;此处恰好 id==1
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		ID           int64  `json:"id"`
		FullName     string `json:"full_name"`
		Stars        int    `json:"stars"`
		Forks        int    `json:"forks"`
		BestDailyRank *int  `json:"best_daily_rank"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, id, body.ID)
	require.Equal(t, "a/x", body.FullName)
	require.Equal(t, 1000, body.Stars)
	require.Equal(t, 50, body.Forks)
	require.NotNil(t, body.BestDailyRank)
	require.Equal(t, 3, *body.BestDailyRank)
}

func TestRepositoryDetailNotFound(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/99999")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRepositoryDetailBadID(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/abc")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRepositoryDetailNullBestRankWhenNeverRanked(t *testing.T) {
	s, db := newTestServer(t)
	_ = seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	rec := doGET(t, s, "/api/v1/repositories/1")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		BestDailyRank *int `json:"best_daily_rank"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Nil(t, body.BestDailyRank) // 从未上榜 → null
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run TestRepositoryDetail -v`
Expected: FAIL — `s.handleRepository` undefined（编译失败)。

- [ ] **Step 4: 实现 handler**

`internal/api/repositories.go`:
```go
package api

import (
	"net/http"
	"strconv"
)

type repositoryDetailDTO struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	Homepage      string `json:"homepage"`
	HTMLURL       string `json:"html_url"`
	OwnerAvatar   string `json:"owner_avatar"`
	Stars         int    `json:"stars"`
	Forks         int    `json:"forks"`
	OpenIssues    int    `json:"open_issues"`
	Watchers      int    `json:"watchers"`
	RepoCreatedAt string `json:"repo_created_at"`
	BestDailyRank *int   `json:"best_daily_rank"`
}

// parseRepoID 从路径取出 {id} 并解析为 int64;失败返回 ok=false(调用方回 400)。
func parseRepoID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (s *Server) handleRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRepoID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repository id")
		return
	}
	repo, found, err := s.db.GetRepositoryByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}

	var bestRank *int
	if br, has, err := s.db.BestDailyRank(id); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	} else if has {
		bestRank = &br
	}

	writeJSON(w, http.StatusOK, repositoryDetailDTO{
		ID: repo.ID, FullName: repo.FullName, Owner: repo.Owner, Name: repo.Name,
		Description: repo.Description, Language: repo.Language, Homepage: repo.Homepage,
		HTMLURL: repo.HTMLURL, OwnerAvatar: repo.OwnerAvatar,
		Stars: repo.Stars, Forks: repo.Forks, OpenIssues: repo.OpenIssues, Watchers: repo.Watchers,
		RepoCreatedAt: repo.RepoCreatedAt, BestDailyRank: bestRank,
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/api/...`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/api/repositories.go internal/api/repositories_test.go internal/api/server.go
git commit -m "$(printf 'feat(api): GET /api/v1/repositories/{id} detail with best daily rank\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 5: api — /repositories/{id}/snapshots 与 /rankings

**Files:**
- Modify: `internal/api/repositories.go`
- Modify: `internal/api/server.go`(注册路由)
- Modify: `internal/api/repositories_test.go`

- [ ] **Step 1: 注册路由**

在 `internal/api/server.go` 的 `Routes()` 里,`repositories/{id}` 行之后加两行:
```go
	mux.HandleFunc("GET /api/v1/repositories/{id}/snapshots", s.handleRepositorySnapshots)
	mux.HandleFunc("GET /api/v1/repositories/{id}/rankings", s.handleRepositoryRankings)
```
即把:
```go
	mux.HandleFunc("GET /api/v1/repositories/{id}", s.handleRepository)
	return mux
```
替换为:
```go
	mux.HandleFunc("GET /api/v1/repositories/{id}", s.handleRepository)
	mux.HandleFunc("GET /api/v1/repositories/{id}/snapshots", s.handleRepositorySnapshots)
	mux.HandleFunc("GET /api/v1/repositories/{id}/rankings", s.handleRepositoryRankings)
	return mux
```

- [ ] **Step 2: 追加失败的测试**

在 `internal/api/repositories_test.go` 末尾追加:
```go
func TestRepositorySnapshotsEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id, Date: "2026-06-09", Stars: 130, StarDelta: 30}))
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id, Date: "2026-06-10", Stars: 150, StarDelta: 20}))

	rec := doGET(t, s, "/api/v1/repositories/1/snapshots")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		RepositoryID int64 `json:"repository_id"`
		Snapshots    []struct {
			Date      string `json:"date"`
			Stars     int    `json:"stars"`
			StarDelta int    `json:"star_delta"`
		} `json:"snapshots"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, id, body.RepositoryID)
	require.Len(t, body.Snapshots, 2)
	require.Equal(t, "2026-06-09", body.Snapshots[0].Date)
	require.Equal(t, 130, body.Snapshots[0].Stars)

	// 区间过滤 + 坏日期
	rec = doGET(t, s, "/api/v1/repositories/1/snapshots?from=2026-06-10")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Snapshots, 1)

	rec = doGET(t, s, "/api/v1/repositories/1/snapshots?from=bad")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRepositoryRankingsEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	id := seedRepoWithMetrics(t, db, 1, "R1", "a/x", "Go", 1000, 50, 5)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []store.Ranking{{RepositoryID: id, Rank: 2, Score: 0.8, StarDelta: 20, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []store.Ranking{{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 30, Language: "Go"}}))

	rec := doGET(t, s, "/api/v1/repositories/1/rankings")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		RepositoryID int64 `json:"repository_id"`
		Rankings     []struct {
			Period    string  `json:"period"`
			Date      string  `json:"date"`
			Rank      int     `json:"rank"`
			Score     float64 `json:"score"`
			StarDelta int     `json:"star_delta"`
		} `json:"rankings"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, id, body.RepositoryID)
	require.Len(t, body.Rankings, 2)
}

func TestRepositorySnapshotsBadID(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/repositories/abc/snapshots")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run "TestRepositorySnapshotsEndpoint|TestRepositoryRankingsEndpoint|TestRepositorySnapshotsBadID" -v`
Expected: FAIL — handler 未定义。

- [ ] **Step 4: 实现 handler**

在 `internal/api/repositories.go` 末尾追加(注意:文件已 import `net/http`、`strconv`;需要新增 `time` 用于日期校验):
先把文件顶部 import 改为:
```go
import (
	"net/http"
	"strconv"
	"time"
)
```
再追加:
```go
type snapshotDTO struct {
	Date       string `json:"date"`
	Stars      int    `json:"stars"`
	Forks      int    `json:"forks"`
	OpenIssues int    `json:"open_issues"`
	Watchers   int    `json:"watchers"`
	StarDelta  int    `json:"star_delta"`
}

type snapshotsResponseDTO struct {
	RepositoryID int64         `json:"repository_id"`
	Snapshots    []snapshotDTO `json:"snapshots"`
}

func (s *Server) handleRepositorySnapshots(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRepoID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repository id")
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	for _, d := range []string{from, to} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				writeError(w, http.StatusBadRequest, "invalid date (use YYYY-MM-DD)")
				return
			}
		}
	}

	snaps, err := s.db.RepositorySnapshots(id, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]snapshotDTO, 0, len(snaps))
	for _, sn := range snaps {
		items = append(items, snapshotDTO{
			Date: sn.Date, Stars: sn.Stars, Forks: sn.Forks,
			OpenIssues: sn.OpenIssues, Watchers: sn.Watchers, StarDelta: sn.StarDelta,
		})
	}
	writeJSON(w, http.StatusOK, snapshotsResponseDTO{RepositoryID: id, Snapshots: items})
}

type rankingHistoryDTO struct {
	Period    string  `json:"period"`
	Date      string  `json:"date"`
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	StarDelta int     `json:"star_delta"`
}

type rankingsResponseDTO struct {
	RepositoryID int64               `json:"repository_id"`
	Rankings     []rankingHistoryDTO `json:"rankings"`
}

func (s *Server) handleRepositoryRankings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRepoID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repository id")
		return
	}
	hist, err := s.db.RepositoryRankingHistory(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]rankingHistoryDTO, 0, len(hist))
	for _, h := range hist {
		items = append(items, rankingHistoryDTO{
			Period: h.Period, Date: h.Date, Rank: h.Rank, Score: h.Score, StarDelta: h.StarDelta,
		})
	}
	writeJSON(w, http.StatusOK, rankingsResponseDTO{RepositoryID: id, Rankings: items})
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/api/...`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/api/repositories.go internal/api/repositories_test.go internal/api/server.go
git commit -m "$(printf 'feat(api): repository snapshots and ranking-history endpoints\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 6: api — GET /api/v1/search

**Files:**
- Create: `internal/api/search.go`
- Modify: `internal/api/server.go`(注册路由)
- Test: `internal/api/search_test.go`

- [ ] **Step 1: 注册路由**

在 `internal/api/server.go` 的 `Routes()` 里,`rankings` 行之后加一行,并在 `return mux` 前:
```go
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
```
即把:
```go
	mux.HandleFunc("GET /api/v1/repositories/{id}/rankings", s.handleRepositoryRankings)
	return mux
```
替换为:
```go
	mux.HandleFunc("GET /api/v1/repositories/{id}/rankings", s.handleRepositoryRankings)
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	return mux
```

- [ ] **Step 2: 写失败的测试**

`internal/api/search_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type searchResp struct {
	Query   string `json:"query"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Total   int    `json:"total"`
	Items   []struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		Stars    int    `json:"stars"`
	} `json:"items"`
}

func TestSearchEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	seedRepoWithMetrics(t, db, 1, "R1", "vercel/next.js", "Go", 1000, 0, 0)
	seedRepoWithMetrics(t, db, 2, "R2", "facebook/react", "JavaScript", 5000)

	rec := doGET(t, s, "/api/v1/search?q=react")
	require.Equal(t, http.StatusOK, rec.Code)
	var body searchResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "react", body.Query)
	require.Equal(t, 1, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, "facebook/react", body.Items[0].FullName)
}

func TestSearchRequiresQuery(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/search")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearchEmptyResultIsEmptyArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/search?q=nomatch")
	require.Equal(t, http.StatusOK, rec.Code)
	var body searchResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Total)
	require.NotNil(t, body.Items)
	require.Len(t, body.Items, 0)
}
```
（`seedRepoWithMetrics` 定义在 repositories_test.go,同包可用;注意它的指标参数 `(stars, forks, issues)`——这里 facebook/react 用 5000 stars,vercel/next.js 用 1000。`mkRepo`?不,用 `seedRepoWithMetrics`。第二个仓库的 desc 默认 "d" 不含 react,full_name 含 react → 命中 1 条。)

- [ ] **Step 3: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run TestSearch -v`
Expected: FAIL — `s.handleSearch` undefined。

- [ ] **Step 4: 实现 handler**

`internal/api/search.go`:
```go
package api

import "net/http"

type searchResponseDTO struct {
	Query   string          `json:"query"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
	Total   int             `json:"total"`
	Items   []repositoryDTO `json:"items"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q is required")
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

	total, err := s.db.SearchRepositoriesCount(query, language)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	repos, err := s.db.SearchRepositoriesByText(query, language, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]repositoryDTO, 0, len(repos))
	for _, rp := range repos {
		items = append(items, repositoryDTO{
			ID: rp.ID, FullName: rp.FullName, Owner: rp.Owner, Name: rp.Name,
			Description: rp.Description, Language: rp.Language, Stars: rp.Stars,
			HTMLURL: rp.HTMLURL, OwnerAvatar: rp.OwnerAvatar,
		})
	}
	writeJSON(w, http.StatusOK, searchResponseDTO{
		Query: query, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
```

- [ ] **Step 5: 运行测试 + 全量 + 冒烟**

Run:
```bash
GOPROXY=off go vet ./...
GOPROXY=off go test ./...
GOPROXY=off go build ./...
# 冒烟:起服务,curl 新端点(空 DB:detail 404,search 缺 q 400,snapshots/rankings 对不存在 id 返回 200+空)
rm -f /tmp/trends_m1b2.db*
DB_PATH=/tmp/trends_m1b2.db API_LISTEN_ADDR=127.0.0.1:18090 GOPROXY=off go run ./cmd/trends >/tmp/m1b2.log 2>&1 &
SRV_PID=$!
sleep 2
for ep in "/api/v1/repositories/1" "/api/v1/repositories/abc" "/api/v1/repositories/1/snapshots" "/api/v1/repositories/1/rankings" "/api/v1/search" "/api/v1/search?q=go"; do
  printf "%-40s -> %s\n" "$ep" "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:18090$ep")"
done
kill -TERM $SRV_PID 2>/dev/null; wait $SRV_PID 2>/dev/null
rm -f /tmp/trends_m1b2.db* /tmp/m1b2.log
```
Expected: vet/build/test 全过。冒烟(空 DB):`/repositories/1`→404、`/repositories/abc`→400、`/repositories/1/snapshots`→200、`/repositories/1/rankings`→200、`/search`→400、`/search?q=go`→200。若沙箱禁回环,以 handler 测试为准并说明。

- [ ] **Step 6: 提交**

```bash
git add internal/api/search.go internal/api/search_test.go internal/api/server.go
git commit -m "$(printf 'feat(api): GET /api/v1/search full-text repository search\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Self-Review(已核对)

- **Spec 覆盖**:对应 spec §7 的 `/repositories/{id}`(详情 + 最佳名次;话题属 Phase 1,不含)、`/repositories/{id}/snapshots`(曲线数据)、`/repositories/{id}/rankings`(历史排名)、`/search`(q + 语言 + 分页)。`best_rank` 用 `trending_rankings` 现算(M0 表无 best_rank 列)。
- **占位符**:无 TBD/TODO;每步均有完整代码与命令。
- **类型一致性**:store 侧 `GetRepositoryByID`/`BestDailyRank`/`RepositorySnapshots`/`RankingHistory`/`RepositoryRankingHistory`/`SearchRepositoriesByText`/`SearchRepositoriesCount`;api 侧 `repositoryDetailDTO`/`snapshotDTO`/`snapshotsResponseDTO`/`rankingHistoryDTO`/`rankingsResponseDTO`/`searchResponseDTO`、复用 `repositoryDTO`/`atoiDefault`/`parseRepoID`。`repoSelectColumns`/`scanRepo`/`scanRepoRows` 复用 M0 既有(同包)。
- **路由特异性**:Go 1.22+ ServeMux 中 `/repositories/{id}` 与 `/repositories/{id}/snapshots`、`/repositories/{id}/rankings` 段数不同,互不冲突(更具体者匹配)。
- **安全**:搜索 LIKE 模式 `%q%` 作为绑定参数(防注入);`{id}` 解析失败/≤0 → 400;不存在 → 404;`per_page` 钳制 [1,100];日期格式校验。
- **空集**:search/snapshots/rankings 列表均 `make([]T,0,..)` → `[]` 而非 null。snapshots/rankings 对不存在的 id 返回 200 + 空数组(子资源,空响应合理);仅 `/repositories/{id}` 详情对不存在 id 返回 404。

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-03-m1b2-repo-detail-search.md`.
