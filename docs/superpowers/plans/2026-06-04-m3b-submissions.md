# M3b 仓库提交收录 实现计划

> 控制器直实现(subagent 基础设施不稳),逐任务 TDD + 验证 + 独立提交。

**Goal:** 用户可提交一个 `owner/repo` 请求收录:`POST /api/v1/submissions`(校验格式 + 每 IP 限流 + 落库 pending),前端 `/submit` 表单。Discovery 作业处理 pending 提交——用 GitHub API 拉该仓库,存在则 upsert 进宇宙并标记 accepted,不存在则 rejected。

**Architecture:** 新增 `submissions` 表(迁移 0003)+ store 增删查方法;`github.Client` 增加 `FetchRepository(fullName)`(REST `GET /repos/{owner}/{repo}`);`api` 增加 POST handler + 一个极简内存限流器;`ingest` 增加 `RunSubmissions` 作业(在 discovery cron 内调用);前端加 `/submit` 页 + client 函数。

**Tech Stack:** Go 标准库 · React/TS · Vitest。无新增第三方依赖(限流器自写)。

> **依赖:** M0–M2 + M3a 在 `main`。已有 `store.UpsertRepository`、`github.Client`(searchResponse item 映射逻辑可复用)、`ingest.Discoverer`、api `Server`/`writeJSON`/`writeError`、前端 api client。离线:`GOPROXY=off go test ./cmd/... ./internal/...`。

---

## Task 1: store — submissions 表 + 方法

**Files:** `internal/store/migrations/0003_submissions.sql`、`internal/store/submission.go`、`internal/store/submission_test.go`

- [ ] **Step 1: 迁移** `0003_submissions.sql`:
```sql
CREATE TABLE submissions (
    id           INTEGER PRIMARY KEY,
    full_name    TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'pending',  -- pending|accepted|rejected
    submitted_ip TEXT,
    note         TEXT,
    created_at   TEXT    NOT NULL
);
CREATE INDEX idx_submissions_status ON submissions(status);
```

- [ ] **Step 2: 失败测试** `submission_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInsertAndListPendingSubmissions(t *testing.T) {
	db := newTestDB(t)
	id, err := db.InsertSubmission("octocat/hello", "1.2.3.4")
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Len(t, pend, 1)
	require.Equal(t, "octocat/hello", pend[0].FullName)
	require.Equal(t, "pending", pend[0].Status)
}

func TestMarkSubmission(t *testing.T) {
	db := newTestDB(t)
	id, err := db.InsertSubmission("a/b", "")
	require.NoError(t, err)
	require.NoError(t, db.MarkSubmission(id, "accepted", ""))

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Empty(t, pend) // 已不再 pending
}
```

- [ ] **Step 3: 实现** `submission.go`:
```go
package store

type Submission struct {
	ID          int64
	FullName    string
	Status      string
	SubmittedIP string
	Note        string
	CreatedAt   string
}

// InsertSubmission 记录一条 pending 提交,返回 id。
func (d *DB) InsertSubmission(fullName, ip string) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO submissions (full_name, status, submitted_ip, created_at) VALUES (?, 'pending', ?, ?)`,
		fullName, ip, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListPendingSubmissions 返回最多 limit 条 pending 提交(按 id 升序)。
func (d *DB) ListPendingSubmissions(limit int) ([]Submission, error) {
	rows, err := d.db.Query(
		`SELECT id, full_name, status, COALESCE(submitted_ip,''), COALESCE(note,''), created_at
FROM submissions WHERE status='pending' ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Submission
	for rows.Next() {
		var s Submission
		if err := rows.Scan(&s.ID, &s.FullName, &s.Status, &s.SubmittedIP, &s.Note, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// MarkSubmission 更新提交状态(accepted/rejected)与备注。
func (d *DB) MarkSubmission(id int64, status, note string) error {
	_, err := d.db.Exec(`UPDATE submissions SET status=?, note=? WHERE id=?`, status, note, id)
	return err
}
```
(`nowUTC()` 在 repository.go,同包。`LastInsertId` 对纯 INSERT 可靠。)

- [ ] **Step 4:** `GOPROXY=off go test ./internal/store/...` → PASS。提交。

---

## Task 2: github — FetchRepository(单仓库查询)

**Files:** 改 `internal/github/client.go`、`internal/github/client_test.go`

- [ ] **Step 1: 失败测试**(追加到 client_test.go):
```go
func TestFetchRepositoryFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/octo/a", r.URL.Path)
		w.Write([]byte(`{"id":111,"node_id":"R_111","full_name":"octo/a","name":"a",
		  "owner":{"login":"octo","avatar_url":"https://av/1"},"description":"d","language":"Go",
		  "homepage":"","html_url":"https://github.com/octo/a","stargazers_count":150,"forks_count":20,
		  "open_issues_count":3,"archived":false,"created_at":"2024-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	repo, found, err := c.FetchRepository(context.Background(), "octo/a")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(111), repo.GitHubID)
	require.Equal(t, "R_111", repo.NodeID)
	require.Equal(t, "octo/a", repo.FullName)
}

func TestFetchRepositoryNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	_, found, err := c.FetchRepository(context.Background(), "no/such")
	require.NoError(t, err)
	require.False(t, found)
}
```

- [ ] **Step 2:** `GOPROXY=off go test ./internal/github/ -run TestFetchRepository` → FAIL。

- [ ] **Step 3: 实现**(追加到 client.go;复用已有的 `searchItem` 字段结构——实际上 search 用的是内联匿名结构,这里定义一个具名 `restRepo` 复用映射)。追加:
```go
// restRepo 是 REST 单仓库/搜索项的字段子集。
type restRepo struct {
	ID       int64  `json:"id"`
	NodeID   string `json:"node_id"`
	FullName string `json:"full_name"`
	Name     string `json:"name"`
	Owner    struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
	Description     string `json:"description"`
	Language        string `json:"language"`
	Homepage        string `json:"homepage"`
	HTMLURL         string `json:"html_url"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	OpenIssuesCount int    `json:"open_issues_count"`
	Archived        bool   `json:"archived"`
	CreatedAt       string `json:"created_at"`
}

func (rr restRepo) toRepository() store.Repository {
	return store.Repository{
		GitHubID: rr.ID, NodeID: rr.NodeID, FullName: rr.FullName, Owner: rr.Owner.Login, Name: rr.Name,
		Description: rr.Description, Language: rr.Language, Homepage: rr.Homepage, HTMLURL: rr.HTMLURL,
		OwnerAvatar: rr.Owner.AvatarURL, Stars: rr.StargazersCount, Forks: rr.ForksCount,
		OpenIssues: rr.OpenIssuesCount, IsArchived: rr.Archived, RepoCreatedAt: rr.CreatedAt,
	}
}

// FetchRepository 用 REST GET /repos/{fullName} 查单个仓库;404 → found=false。
func (c *Client) FetchRepository(ctx context.Context, fullName string) (store.Repository, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.restBase+"/repos/"+fullName, nil)
	if err != nil {
		return store.Repository{}, false, err
	}
	c.auth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return store.Repository{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return store.Repository{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return store.Repository{}, false, fmt.Errorf("github repos: status %d: %s", resp.StatusCode, b)
	}
	var rr restRepo
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return store.Repository{}, false, err
	}
	return rr.toRepository(), true, nil
}
```

- [ ] **Step 4:** `GOPROXY=off go test ./internal/github/...` → PASS。提交。

---

## Task 3: api — POST /api/v1/submissions(+ 内存限流)

**Files:** `internal/api/ratelimit.go`、`internal/api/submissions.go`、`internal/api/submissions_test.go`、改 `internal/api/server.go`(限流器字段 + 路由)

- [ ] **Step 1: 限流器 + 失败测试**

`internal/api/ratelimit.go`:
```go
package api

import (
	"sync"
	"time"
)

// rateLimiter 是极简的滑动窗口内存限流器(每 key 在 window 内最多 max 次)。
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: make(map[string][]time.Time), max: max, window: window}
}

func (rl *rateLimiter) allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := now.Add(-rl.window)
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.max {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}
```

`internal/api/submissions_test.go`:
```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func TestSubmissionAccepted(t *testing.T) {
	s, db := newTestServer(t)
	rec := postJSON(t, s, "/api/v1/submissions", `{"full_name":"octocat/hello"}`)
	require.Equal(t, http.StatusAccepted, rec.Code)

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Len(t, pend, 1)
	require.Equal(t, "octocat/hello", pend[0].FullName)
}

func TestSubmissionRejectsBadFullName(t *testing.T) {
	s, _ := newTestServer(t)
	for _, bad := range []string{`{"full_name":"noslash"}`, `{"full_name":"a/b/c"}`, `{"full_name":""}`, `not json`} {
		rec := postJSON(t, s, "/api/v1/submissions", bad)
		require.Equal(t, http.StatusBadRequest, rec.Code, bad)
	}
}

func TestSubmissionRateLimited(t *testing.T) {
	s, _ := newTestServer(t)
	s.submitLimiter = newRateLimiter(1, time.Hour) // 测试用低阈值
	require.Equal(t, http.StatusAccepted, postJSON(t, s, "/api/v1/submissions", `{"full_name":"a/b"}`).Code)
	require.Equal(t, http.StatusTooManyRequests, postJSON(t, s, "/api/v1/submissions", `{"full_name":"a/c"}`).Code)
}
```

- [ ] **Step 2:** `GOPROXY=off go test ./internal/api/ -run TestSubmission` → FAIL。

- [ ] **Step 3: 实现** — 改 `Server` 加字段并在 `NewServer` 初始化;加路由;写 handler。

`server.go`:在 `Server` struct 加 `submitLimiter *rateLimiter`;`NewServer` 里 `submitLimiter: newRateLimiter(20, time.Hour)`(需 import `time`);`Routes()` 加 `mux.HandleFunc("POST /api/v1/submissions", s.handleSubmit)`。

`submissions.go`:
```go
package api

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
)

var fullNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.submitLimiter.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "too many submissions, try later")
		return
	}
	var body struct {
		FullName string `json:"full_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !fullNameRe.MatchString(body.FullName) {
		writeError(w, http.StatusBadRequest, "full_name must be owner/repo")
		return
	}
	id, err := s.db.InsertSubmission(body.FullName, ip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "pending"})
}
```

- [ ] **Step 4:** `GOPROXY=off go test ./internal/api/...` → PASS。提交。

---

## Task 4: ingest — RunSubmissions + 接入 discovery cron

**Files:** `internal/ingest/submissions.go`、`internal/ingest/submissions_test.go`、改 `cmd/trends/main.go`

- [ ] **Step 1: 失败测试** `submissions_test.go`(给 fakeClient 加 FetchRepository,或用新 fake):
```go
package ingest

import (
	"context"
	"testing"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

type fakeFetcher struct {
	repos map[string]store.Repository // fullName -> repo;不在表中视为 not found
}

func (f *fakeFetcher) FetchRepository(_ context.Context, fullName string) (store.Repository, bool, error) {
	r, ok := f.repos[fullName]
	return r, ok, nil
}

var _ = github.RepoMetrics{} // 保持 github 被引用(如未直接用可删)

func TestRunSubmissionsAcceptsAndRejects(t *testing.T) {
	db := newTestDB(t)
	good, err := db.InsertSubmission("octo/good", "ip")
	require.NoError(t, err)
	bad, err := db.InsertSubmission("no/such", "ip")
	require.NoError(t, err)

	ff := &fakeFetcher{repos: map[string]store.Repository{
		"octo/good": {GitHubID: 1, NodeID: "R1", FullName: "octo/good", Owner: "octo", Name: "good", HTMLURL: "u"},
	}}
	require.NoError(t, RunSubmissions(context.Background(), db, ff, 10))

	// good → 已收录(repositories 里有)+ submission accepted;bad → rejected
	_, err = db.GetRepositoryByGitHubID(1)
	require.NoError(t, err)
	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Empty(t, pend) // 两条都处理掉了
	_ = good
	_ = bad
}
```

- [ ] **Step 2:** `GOPROXY=off go test ./internal/ingest/ -run TestRunSubmissions` → FAIL。

- [ ] **Step 3: 实现** `submissions.go`:
```go
package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/store"
)

// RepoFetcher 是处理提交所需的 GitHub 能力子集。
type RepoFetcher interface {
	FetchRepository(ctx context.Context, fullName string) (store.Repository, bool, error)
}

// RunSubmissions 处理 pending 提交:存在的仓库 upsert 进宇宙并标记 accepted,不存在的标记 rejected。
func RunSubmissions(ctx context.Context, db *store.DB, gh RepoFetcher, limit int) error {
	subs, err := db.ListPendingSubmissions(limit)
	if err != nil {
		return err
	}
	accepted, rejected := 0, 0
	for _, sub := range subs {
		if err := ctx.Err(); err != nil {
			return err
		}
		repo, found, err := gh.FetchRepository(ctx, sub.FullName)
		if err != nil {
			return err // 瞬时错误:留待下次重试
		}
		if !found {
			if err := db.MarkSubmission(sub.ID, "rejected", "repository not found"); err != nil {
				return err
			}
			rejected++
			continue
		}
		if _, err := db.UpsertRepository(repo); err != nil {
			return err
		}
		if err := db.MarkSubmission(sub.ID, "accepted", ""); err != nil {
			return err
		}
		accepted++
	}
	slog.Info("submissions processed", "accepted", accepted, "rejected", rejected, "pending_seen", len(subs))
	return nil
}
```

- [ ] **Step 4: 接入 main** — `cmd/trends/main.go`:`runDiscovery` 末尾在 search 发现之后调用 `ingest.RunSubmissions(ctx, db, gh, 200)`(同一个 30 分钟 ctx;`gh` 即 `*github.Client`,已实现 `FetchRepository` → 满足 `ingest.RepoFetcher`)。即把:
```go
		_, err := ingest.RunDiscovery(ctx, db, gh, discoveryQueries, 10)
		return err
```
改为:
```go
		if _, err := ingest.RunDiscovery(ctx, db, gh, discoveryQueries, 10); err != nil {
			return err
		}
		return ingest.RunSubmissions(ctx, db, gh, 200)
```

- [ ] **Step 5:** `GOPROXY=off go test ./internal/ingest/...` + `GOPROXY=off go build ./cmd/... ./internal/...` → PASS。提交。

---

## Task 5: 前端 /submit 页 + e2e + 收尾

**Files:** 改 `web/src/api/client.ts`(+ `submitRepository`)、`web/src/pages/Submit.tsx`、`Submit.test.tsx`、改 `App.tsx`(路由)、`Layout.tsx`(导航链接)

- [ ] **Step 1:** client.ts 加:
```ts
export async function submitRepository(fullName: string): Promise<{ id: number; status: string }> {
  const res = await fetch('/api/v1/submissions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ full_name: fullName }),
  })
  if (!res.ok) {
    let msg = `request failed: ${res.status}`
    try { const b = (await res.json()) as { error?: string }; if (b?.error) msg = b.error } catch { /* ignore */ }
    throw new Error(msg)
  }
  return res.json() as Promise<{ id: number; status: string }>
}
```

- [ ] **Step 2: 失败测试** `Submit.test.tsx` — mock `client.submitRepository`,渲染,填 `owner/repo` 提交,断言显示成功提示;mock 抛错断言显示错误。

- [ ] **Step 3: 实现** `Submit.tsx`(受控输入 + 提交 → 成功/错误态);`App.tsx` 加 `<Route path="/submit" element={<Submit />} />`;`Layout.tsx` 导航加「提交」链接。

- [ ] **Step 4:** `cd web && npm run test` + `npm run build` → PASS。

- [ ] **Step 5: e2e** — `make web && go build`,起服务,curl:
  - `POST /api/v1/submissions {"full_name":"octocat/Hello-World"}` → 202。
  - `POST` 非法 `{"full_name":"x"}` → 400。
  清理 embed 产物。提交。

---

## Self-Review(已核对)
- **Spec 覆盖**:spec §6.4(submissions 表)、§7(`POST /api/v1/submissions` + 防滥用限流)、§8.1(`/submit` 页)、§4.1(用户提交作为宇宙发现来源之一 → RunSubmissions 接入 discovery)。
- **安全**:full_name 正则白名单(防注入/路径滥用);请求体 `MaxBytesReader` 限 4KB;每 IP 内存限流(默认 20/小时);提交不直接进宇宙,经 GitHub 校验存在才 upsert。
- **类型一致**:`store.Submission`/Insert/ListPending/Mark;`github.FetchRepository` 返回 `store.Repository`;`ingest.RepoFetcher` 由 `*github.Client` 满足;前端 `submitRepository`。
- **限流可测**:`rateLimiter` 逻辑 + handler 429 路径(测试里把 `s.submitLimiter` 换成低阈值)。

## Execution Handoff
Plan complete and saved to `docs/superpowers/plans/2026-06-04-m3b-submissions.md`.
