# M0 数据地基 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 Go 搭起数据采集地基——发现并维护「仓库宇宙」、每日给宇宙内仓库拍 star/fork/issue 快照并算出日增量,全部落进内嵌 SQLite。

**Architecture:** 单一 Go 二进制内含 cron 调度器,跑两个作业:Discovery(用 GitHub REST Search 按 star 区间切片发现仓库并 upsert)与 Snapshot(用 GitHub GraphQL `nodes(ids:)` 批量拉取指标、计算 `star_delta`、写日快照并更新冗余字段)。存储层用 `database/sql` + `modernc.org/sqlite`(纯 Go,无 cgo),WAL 模式,迁移用 `embed` 内嵌 SQL 顺序执行。

**Tech Stack:** Go 1.23 · `modernc.org/sqlite` · `github.com/robfig/cron/v3` · `github.com/stretchr/testify` · GitHub REST/GraphQL API

> **对 spec §6 的一处细化**:`repositories` 增加 `node_id TEXT` 列。Snapshot 作业用 GraphQL `nodes(ids: [ID!]!)` 一次批量拉取多个仓库,这需要每个仓库的 GraphQL 全局 node ID(REST Search 返回的 `node_id`)。
> **对 spec §3.3 的一处细化**:迁移 SQL 放在 `internal/store/migrations/`(而非顶层 `migrations/`),因为 `go:embed` 不能引用父目录。

---

## File Structure

| 文件 | 职责 |
|---|---|
| `go.mod` | 模块定义与依赖 |
| `internal/config/config.go` | 从环境变量加载配置 |
| `internal/config/config_test.go` | 配置加载测试 |
| `internal/store/store.go` | 打开 SQLite(WAL)+ 运行迁移 |
| `internal/store/migrations/0001_init.sql` | `repositories` + `repository_snapshots` 建表 |
| `internal/store/repository.go` | 仓库模型与 upsert/list/update 方法 |
| `internal/store/repository_test.go` | 仓库存储测试 |
| `internal/store/snapshot.go` | 快照插入与「上一次快照」查询 |
| `internal/store/snapshot_test.go` | 快照存储测试 |
| `internal/github/client.go` | GitHub 客户端(REST Search + GraphQL 批量)+ 接口 |
| `internal/github/client_test.go` | 用 httptest 模拟 GitHub 的客户端测试 |
| `internal/ingest/discovery.go` | Discovery 作业 |
| `internal/ingest/discovery_test.go` | Discovery 作业测试(mock client) |
| `internal/ingest/snapshot.go` | Snapshot 作业(算 delta、写库) |
| `internal/ingest/snapshot_test.go` | Snapshot 作业测试(mock client) |
| `internal/scheduler/scheduler.go` | cron 注册两个作业 |
| `cmd/trends/main.go` | 装配 config/store/client/scheduler |
| `litestream.yml` | Litestream 备份配置(运维) |

设计边界:`github` 包只懂「怎么跟 GitHub 说话」并返回纯数据;`store` 包只懂「怎么读写 SQLite」;`ingest` 包编排两者、含业务逻辑(切片查询、算 delta);三者通过接口解耦,作业测试用 mock client + 临时 DB,不触网。

---

## Task 1: 项目骨架与配置

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: 初始化模块并加依赖**

Run:
```bash
cd /Users/matthew/projects/meirongdev/trends
go mod init github.com/meirongdev/trends
go get modernc.org/sqlite@latest
go get github.com/robfig/cron/v3@latest
go get github.com/stretchr/testify@latest
```
Expected: 生成 `go.mod` / `go.sum`,无报错。

- [ ] **Step 2: 写失败的配置测试**

`internal/config/config_test.go`:
```go
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
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/config/...`
Expected: FAIL — `undefined: Load`(包还没有实现)。

- [ ] **Step 4: 实现配置加载**

`internal/config/config.go`:
```go
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
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/config/...`
Expected: PASS（ok）。

- [ ] **Step 6: 提交**

```bash
git init        # 仓库尚未初始化,首次提交前执行一次即可
git add go.mod go.sum internal/config/
git commit -m "feat(config): load configuration from environment"
```

---

## Task 2: SQLite 存储 — 打开与迁移

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/migrations/0001_init.sql`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: 写迁移 SQL**

`internal/store/migrations/0001_init.sql`:
```sql
CREATE TABLE repositories (
    id              INTEGER PRIMARY KEY,
    github_id       INTEGER NOT NULL UNIQUE,
    node_id         TEXT    NOT NULL,
    full_name       TEXT    NOT NULL UNIQUE,
    owner           TEXT    NOT NULL,
    name            TEXT    NOT NULL,
    description     TEXT,
    language        TEXT,
    homepage        TEXT,
    html_url        TEXT    NOT NULL,
    owner_avatar    TEXT,
    stars           INTEGER NOT NULL DEFAULT 0,
    forks           INTEGER NOT NULL DEFAULT 0,
    open_issues     INTEGER NOT NULL DEFAULT 0,
    watchers        INTEGER NOT NULL DEFAULT 0,
    is_archived     INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 1,
    repo_created_at TEXT,
    first_seen_at   TEXT    NOT NULL,
    last_synced_at  TEXT
);
CREATE INDEX idx_repos_language ON repositories(language);
CREATE INDEX idx_repos_active   ON repositories(is_active);
CREATE INDEX idx_repos_stars    ON repositories(stars DESC);

CREATE TABLE repository_snapshots (
    repository_id   INTEGER NOT NULL REFERENCES repositories(id),
    snapshot_date   TEXT    NOT NULL,
    stars           INTEGER NOT NULL,
    forks           INTEGER NOT NULL,
    open_issues     INTEGER NOT NULL,
    watchers        INTEGER NOT NULL DEFAULT 0,
    star_delta      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (repository_id, snapshot_date)
);
CREATE INDEX idx_snapshots_date ON repository_snapshots(snapshot_date);
```

- [ ] **Step 2: 写失败的迁移测试**

`internal/store/store_test.go`:
```go
package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenRunsMigrations(t *testing.T) {
	db := newTestDB(t)

	// 表存在:能查询且返回 0 行
	var count int
	err := db.SQL().QueryRow(`SELECT COUNT(*) FROM repositories`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	err = db.SQL().QueryRow(`SELECT COUNT(*) FROM repository_snapshots`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db1, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, db1.Close())

	// 第二次打开同一文件不应因重复建表而报错
	db2, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, db2.Close())
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/store/...`
Expected: FAIL — `undefined: Open` / `undefined: DB`。

- [ ] **Step 4: 实现 store 打开与迁移**

`internal/store/store.go`:
```go
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// DB 包裹 *sql.DB,作为存储层句柄。
type DB struct {
	db *sql.DB
}

// SQL 暴露底层连接(供包内方法与测试使用)。
func (d *DB) SQL() *sql.DB { return d.db }

func (d *DB) Close() error { return d.db.Close() }

// Open 打开 SQLite(WAL 模式)并应用所有迁移。
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)",
		path,
	)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &DB{db: sqlDB}, nil
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var applied string
		err := db.QueryRow(`SELECT version FROM schema_migrations WHERE version=?`, name).Scan(&applied)
		if err == nil {
			continue // 已应用
		}
		if err != sql.ErrNoRows {
			return err
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, name); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/store/store.go internal/store/store_test.go internal/store/migrations/
git commit -m "feat(store): open sqlite with WAL and run embedded migrations"
```

---

## Task 3: 仓库存储方法

**Files:**
- Create: `internal/store/repository.go`
- Test: `internal/store/repository_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/store/repository_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleRepo() Repository {
	return Repository{
		GitHubID:      111,
		NodeID:        "R_node_111",
		FullName:      "octocat/hello",
		Owner:         "octocat",
		Name:          "hello",
		Description:   "demo",
		Language:      "Go",
		HTMLURL:       "https://github.com/octocat/hello",
		OwnerAvatar:   "https://avatars/1",
		RepoCreatedAt: "2024-01-01T00:00:00Z",
	}
}

func TestUpsertInsertsThenUpdates(t *testing.T) {
	db := newTestDB(t)

	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// 再次 upsert 同 github_id:更新描述,id 不变
	r := sampleRepo()
	r.Description = "updated"
	id2, err := db.UpsertRepository(r)
	require.NoError(t, err)
	require.Equal(t, id, id2)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, "updated", got.Description)
	require.True(t, got.IsActive)
}

func TestListActiveRepositories(t *testing.T) {
	db := newTestDB(t)
	_, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	repos, err := db.ListActiveRepositories()
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "R_node_111", repos[0].NodeID)
}

func TestUpdateRepositoryMetrics(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	err = db.UpdateRepositoryMetrics(111, 500, 50, 5, 12, "2026-06-03T00:00:00Z")
	require.NoError(t, err)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, int64(id), int64(got.ID))
	require.Equal(t, 500, got.Stars)
	require.Equal(t, 50, got.Forks)
	require.Equal(t, 5, got.OpenIssues)
	require.Equal(t, 12, got.Watchers)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/store/ -run TestUpsert -v`
Expected: FAIL — `undefined: Repository` 等。

- [ ] **Step 3: 实现仓库模型与方法**

`internal/store/repository.go`:
```go
package store

import (
	"database/sql"
	"time"
)

type Repository struct {
	ID            int64
	GitHubID      int64
	NodeID        string
	FullName      string
	Owner         string
	Name          string
	Description   string
	Language      string
	Homepage      string
	HTMLURL       string
	OwnerAvatar   string
	Stars         int
	Forks         int
	OpenIssues    int
	Watchers      int
	IsArchived    bool
	IsActive      bool
	RepoCreatedAt string
	FirstSeenAt   string
	LastSyncedAt  string
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// UpsertRepository 按 github_id 插入或更新元数据,返回内部 id。
// first_seen_at 仅在插入时设置;stars/forks 等指标由 UpdateRepositoryMetrics 维护。
func (d *DB) UpsertRepository(r Repository) (int64, error) {
	_, err := d.db.Exec(`
INSERT INTO repositories
  (github_id, node_id, full_name, owner, name, description, language, homepage,
   html_url, owner_avatar, is_archived, repo_created_at, first_seen_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(github_id) DO UPDATE SET
  node_id        = excluded.node_id,
  full_name      = excluded.full_name,
  owner          = excluded.owner,
  name           = excluded.name,
  description    = excluded.description,
  language       = excluded.language,
  homepage       = excluded.homepage,
  html_url       = excluded.html_url,
  owner_avatar   = excluded.owner_avatar,
  is_archived    = excluded.is_archived,
  repo_created_at= excluded.repo_created_at
`,
		r.GitHubID, r.NodeID, r.FullName, r.Owner, r.Name, r.Description, r.Language, r.Homepage,
		r.HTMLURL, r.OwnerAvatar, b2i(r.IsArchived), r.RepoCreatedAt, nowUTC(),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.QueryRow(`SELECT id FROM repositories WHERE github_id=?`, r.GitHubID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (d *DB) GetRepositoryByGitHubID(githubID int64) (Repository, error) {
	return scanRepo(d.db.QueryRow(`
SELECT id, github_id, node_id, full_name, owner, name,
       COALESCE(description,''), COALESCE(language,''), COALESCE(homepage,''),
       html_url, COALESCE(owner_avatar,''), stars, forks, open_issues, watchers,
       is_archived, is_active, COALESCE(repo_created_at,''), first_seen_at, COALESCE(last_synced_at,'')
FROM repositories WHERE github_id=?`, githubID))
}

func (d *DB) ListActiveRepositories() ([]Repository, error) {
	rows, err := d.db.Query(`
SELECT id, github_id, node_id, full_name, owner, name,
       COALESCE(description,''), COALESCE(language,''), COALESCE(homepage,''),
       html_url, COALESCE(owner_avatar,''), stars, forks, open_issues, watchers,
       is_archived, is_active, COALESCE(repo_created_at,''), first_seen_at, COALESCE(last_synced_at,'')
FROM repositories WHERE is_active=1`)
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

func (d *DB) UpdateRepositoryMetrics(githubID int64, stars, forks, openIssues, watchers int, syncedAt string) error {
	_, err := d.db.Exec(`
UPDATE repositories
SET stars=?, forks=?, open_issues=?, watchers=?, last_synced_at=?
WHERE github_id=?`,
		stars, forks, openIssues, watchers, syncedAt, githubID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRepo(row *sql.Row) (Repository, error)        { return scanInto(row) }
func scanRepoRows(rows *sql.Rows) (Repository, error)  { return scanInto(rows) }

func scanInto(s rowScanner) (Repository, error) {
	var r Repository
	var archived, active int
	err := s.Scan(
		&r.ID, &r.GitHubID, &r.NodeID, &r.FullName, &r.Owner, &r.Name,
		&r.Description, &r.Language, &r.Homepage, &r.HTMLURL, &r.OwnerAvatar,
		&r.Stars, &r.Forks, &r.OpenIssues, &r.Watchers,
		&archived, &active, &r.RepoCreatedAt, &r.FirstSeenAt, &r.LastSyncedAt,
	)
	if err != nil {
		return Repository{}, err
	}
	r.IsArchived = archived == 1
	r.IsActive = active == 1
	return r, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/repository.go internal/store/repository_test.go
git commit -m "feat(store): upsert/list/update repository methods"
```

---

## Task 4: 快照存储方法

**Files:**
- Create: `internal/store/snapshot.go`
- Test: `internal/store/snapshot_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/store/snapshot_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLastStarsReturnsFalseWhenNone(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	_, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestInsertSnapshotAndLastStars(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.InsertSnapshot(Snapshot{
		RepositoryID: id, Date: "2026-06-02", Stars: 100, Forks: 10, OpenIssues: 2, Watchers: 5, StarDelta: 0,
	}))

	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 100, stars)
}

func TestInsertSnapshotIsIdempotentPerDay(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	s := Snapshot{RepositoryID: id, Date: "2026-06-02", Stars: 100, Forks: 10, OpenIssues: 2, Watchers: 5}
	require.NoError(t, db.InsertSnapshot(s))
	s.Stars = 120 // 同一天重跑应覆盖,不报主键冲突
	require.NoError(t, db.InsertSnapshot(s))

	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 120, stars)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/store/ -run TestInsertSnapshot -v`
Expected: FAIL — `undefined: Snapshot`。

- [ ] **Step 3: 实现快照存储**

`internal/store/snapshot.go`:
```go
package store

import "database/sql"

type Snapshot struct {
	RepositoryID int64
	Date         string // YYYY-MM-DD (UTC)
	Stars        int
	Forks        int
	OpenIssues   int
	Watchers     int
	StarDelta    int
}

// InsertSnapshot 写入(或覆盖)某仓库某天的快照,对同日重跑幂等。
func (d *DB) InsertSnapshot(s Snapshot) error {
	_, err := d.db.Exec(`
INSERT INTO repository_snapshots
  (repository_id, snapshot_date, stars, forks, open_issues, watchers, star_delta)
VALUES (?,?,?,?,?,?,?)
ON CONFLICT(repository_id, snapshot_date) DO UPDATE SET
  stars=excluded.stars, forks=excluded.forks, open_issues=excluded.open_issues,
  watchers=excluded.watchers, star_delta=excluded.star_delta`,
		s.RepositoryID, s.Date, s.Stars, s.Forks, s.OpenIssues, s.Watchers, s.StarDelta)
	return err
}

// LastStars 返回该仓库最近一天快照的 star 数;无快照时 ok=false。
func (d *DB) LastStars(repositoryID int64) (int, bool, error) {
	var stars int
	err := d.db.QueryRow(`
SELECT stars FROM repository_snapshots
WHERE repository_id=?
ORDER BY snapshot_date DESC LIMIT 1`, repositoryID).Scan(&stars)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return stars, true, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/snapshot.go internal/store/snapshot_test.go
git commit -m "feat(store): insert daily snapshots and query last star count"
```

---

## Task 5: GitHub 客户端 — REST Search(发现)

**Files:**
- Create: `internal/github/client.go`
- Test: `internal/github/client_test.go`

- [ ] **Step 1: 写失败的测试(用 httptest 模拟 GitHub)**

`internal/github/client_test.go`:
```go
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchRepositoriesParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/search/repositories", r.URL.Path)
		require.Equal(t, "stars:100..200", r.URL.Query().Get("q"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
		  "items": [
		    {"id": 111, "node_id": "R_111", "full_name": "octo/a", "name": "a",
		     "owner": {"login": "octo", "avatar_url": "https://av/1"},
		     "description": "d", "language": "Go", "homepage": "",
		     "html_url": "https://github.com/octo/a",
		     "stargazers_count": 150, "forks_count": 20, "open_issues_count": 3,
		     "archived": false, "created_at": "2024-01-01T00:00:00Z"}
		  ]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	repos, err := c.SearchRepositories(context.Background(), "stars:100..200", 1)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, int64(111), repos[0].GitHubID)
	require.Equal(t, "R_111", repos[0].NodeID)
	require.Equal(t, "octo/a", repos[0].FullName)
	require.Equal(t, "Go", repos[0].Language)
	require.Equal(t, "https://av/1", repos[0].OwnerAvatar)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/github/...`
Expected: FAIL — `undefined: NewClient`。

- [ ] **Step 3: 实现客户端骨架与 Search**

`internal/github/client.go`:
```go
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/meirongdev/trends/internal/store"
)

// RepoMetrics 是 Snapshot 作业需要的可变指标(见 Task 6)。
type RepoMetrics struct {
	GitHubID   int64
	Stars      int
	Forks      int
	OpenIssues int
	Watchers   int
}

// Client 与 GitHub REST/GraphQL 通信。restBase 与 graphqlURL 在测试中指向 httptest。
type Client struct {
	http       *http.Client
	restBase   string
	graphqlURL string
	tokens     []string
	tokenIdx   uint32
}

func NewClient(restBase, graphqlURL string, tokens []string) *Client {
	return &Client{
		http:       &http.Client{Timeout: 30 * time.Second},
		restBase:   restBase,
		graphqlURL: graphqlURL,
		tokens:     tokens,
	}
}

// nextToken 在多个 token 间轮询;无 token 时返回空串(不带鉴权)。
func (c *Client) nextToken() string {
	if len(c.tokens) == 0 {
		return ""
	}
	i := atomic.AddUint32(&c.tokenIdx, 1)
	return c.tokens[int(i)%len(c.tokens)]
}

func (c *Client) auth(req *http.Request) {
	if tok := c.nextToken(); tok != "" {
		req.Header.Set("Authorization", "bearer "+tok)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
}

type searchResponse struct {
	Items []struct {
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
	} `json:"items"`
}

// SearchRepositories 调 REST /search/repositories,返回该页结果(映射为 store.Repository)。
func (c *Client) SearchRepositories(ctx context.Context, query string, page int) ([]store.Repository, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("sort", "stars")
	q.Set("order", "desc")
	q.Set("per_page", "100")
	q.Set("page", strconv.Itoa(page))

	endpoint := c.restBase + "/search/repositories?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.auth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github search: status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	repos := make([]store.Repository, 0, len(sr.Items))
	for _, it := range sr.Items {
		repos = append(repos, store.Repository{
			GitHubID:      it.ID,
			NodeID:        it.NodeID,
			FullName:      it.FullName,
			Owner:         it.Owner.Login,
			Name:          it.Name,
			Description:   it.Description,
			Language:      it.Language,
			Homepage:      it.Homepage,
			HTMLURL:       it.HTMLURL,
			OwnerAvatar:   it.Owner.AvatarURL,
			Stars:         it.StargazersCount,
			Forks:         it.ForksCount,
			OpenIssues:    it.OpenIssuesCount,
			IsArchived:    it.Archived,
			RepoCreatedAt: it.CreatedAt,
		})
	}
	return repos, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/github/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/github/client.go internal/github/client_test.go
git commit -m "feat(github): REST search client for repository discovery"
```

---

## Task 6: GitHub 客户端 — GraphQL 批量拉取(快照)

**Files:**
- Modify: `internal/github/client.go`
- Modify: `internal/github/client_test.go`

- [ ] **Step 1: 追加失败的测试**

在 `internal/github/client_test.go` 末尾追加:
```go
func TestFetchByNodeIDsParsesMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/graphql", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
		  "data": {
		    "nodes": [
		      {"databaseId": 111, "stargazerCount": 500, "forkCount": 40,
		       "issues": {"totalCount": 7}, "watchers": {"totalCount": 9}},
		      null
		    ],
		    "rateLimit": {"remaining": 4999, "resetAt": "2026-06-03T01:00:00Z"}
		  }
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	metrics, err := c.FetchByNodeIDs(context.Background(), []string{"R_111", "R_dead"})
	require.NoError(t, err)
	require.Len(t, metrics, 1) // null 节点被跳过
	require.Equal(t, int64(111), metrics[0].GitHubID)
	require.Equal(t, 500, metrics[0].Stars)
	require.Equal(t, 40, metrics[0].Forks)
	require.Equal(t, 7, metrics[0].OpenIssues)
	require.Equal(t, 9, metrics[0].Watchers)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/github/ -run TestFetchByNodeIDs -v`
Expected: FAIL — `c.FetchByNodeIDs undefined`。

- [ ] **Step 3: 实现 GraphQL 批量拉取**

在 `internal/github/client.go` 末尾追加(import 块已含 `bytes`?需新增,见下方完整 import 调整):

先在文件顶部 import 块加入 `"bytes"`。然后追加:
```go
const repoMetricsQuery = `query($ids: [ID!]!) {
  nodes(ids: $ids) {
    ... on Repository {
      databaseId
      stargazerCount
      forkCount
      issues(states: OPEN) { totalCount }
      watchers { totalCount }
    }
  }
  rateLimit { remaining resetAt }
}`

type graphqlNode struct {
	DatabaseID     int64 `json:"databaseId"`
	StargazerCount int   `json:"stargazerCount"`
	ForkCount      int   `json:"forkCount"`
	Issues         struct {
		TotalCount int `json:"totalCount"`
	} `json:"issues"`
	Watchers struct {
		TotalCount int `json:"totalCount"`
	} `json:"watchers"`
}

type graphqlResponse struct {
	Data struct {
		Nodes []*graphqlNode `json:"nodes"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// FetchByNodeIDs 用 GraphQL nodes(ids:) 一次批量拉取多个仓库的指标。
// 调用方需保证 len(nodeIDs) <= 100。返回中跳过已删除/无权限的 null 节点。
func (c *Client) FetchByNodeIDs(ctx context.Context, nodeIDs []string) ([]RepoMetrics, error) {
	body, err := json.Marshal(map[string]any{
		"query":     repoMetricsQuery,
		"variables": map[string]any{"ids": nodeIDs},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.auth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github graphql: status %d", resp.StatusCode)
	}

	var gr graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	if len(gr.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", gr.Errors[0].Message)
	}

	out := make([]RepoMetrics, 0, len(gr.Data.Nodes))
	for _, n := range gr.Data.Nodes {
		if n == nil {
			continue
		}
		out = append(out, RepoMetrics{
			GitHubID:   n.DatabaseID,
			Stars:      n.StargazerCount,
			Forks:      n.ForkCount,
			OpenIssues: n.Issues.TotalCount,
			Watchers:   n.Watchers.TotalCount,
		})
	}
	return out, nil
}
```

确保 `client.go` 顶部 import 为:
```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/meirongdev/trends/internal/store"
)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/github/...`
Expected: PASS（两个测试都过）。

- [ ] **Step 5: 提交**

```bash
git add internal/github/client.go internal/github/client_test.go
git commit -m "feat(github): graphql batch fetch of repository metrics"
```

---

## Task 7: Discovery 作业

**Files:**
- Create: `internal/ingest/discovery.go`
- Test: `internal/ingest/discovery_test.go`

- [ ] **Step 1: 写失败的测试(mock client + 临时 DB)**

`internal/ingest/discovery_test.go`:
```go
package ingest

import (
	"context"
	"testing"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// fakeClient 实现 Discoverer(本任务)与 Fetcher(Task 8 追加),供作业测试,不触网。
type fakeClient struct {
	searchByQuery map[string][]store.Repository
	metrics       []github.RepoMetrics
}

func (f *fakeClient) SearchRepositories(_ context.Context, query string, page int) ([]store.Repository, error) {
	if page > 1 {
		return nil, nil // 只有一页
	}
	return f.searchByQuery[query], nil
}

func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunDiscoveryUpsertsAcrossQueries(t *testing.T) {
	db := newTestDB(t)
	fc := &fakeClient{searchByQuery: map[string][]store.Repository{
		"stars:100..200": {{GitHubID: 1, NodeID: "R1", FullName: "a/1", Owner: "a", Name: "1", HTMLURL: "u1"}},
		"stars:200..500": {{GitHubID: 2, NodeID: "R2", FullName: "a/2", Owner: "a", Name: "2", HTMLURL: "u2"}},
	}}

	n, err := RunDiscovery(context.Background(), db, fc, []string{"stars:100..200", "stars:200..500"}, 1)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	repos, err := db.ListActiveRepositories()
	require.NoError(t, err)
	require.Len(t, repos, 2)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/ingest/ -run TestRunDiscovery -v`
Expected: FAIL — `undefined: RunDiscovery` / `Discoverer`。

- [ ] **Step 3: 实现 Discovery 作业**

`internal/ingest/discovery.go`:
```go
package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/store"
)

// Discoverer 是 Discovery 作业依赖的 GitHub 能力子集(便于测试 mock)。
type Discoverer interface {
	SearchRepositories(ctx context.Context, query string, page int) ([]store.Repository, error)
}

// RunDiscovery 遍历每个查询、逐页拉取直到空页或到 maxPages,upsert 所有仓库。
// 返回成功 upsert 的仓库数。
func RunDiscovery(ctx context.Context, db *store.DB, gh Discoverer, queries []string, maxPages int) (int, error) {
	count := 0
	for _, q := range queries {
		for page := 1; page <= maxPages; page++ {
			repos, err := gh.SearchRepositories(ctx, q, page)
			if err != nil {
				return count, err
			}
			if len(repos) == 0 {
				break
			}
			for _, r := range repos {
				if _, err := db.UpsertRepository(r); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	slog.Info("discovery complete", "queries", len(queries), "upserted", count)
	return count, nil
}
```

> 注:测试里的 `fakeClient` 只实现了 `Discoverer` 所需的 `SearchRepositories`,因此能直接作为 `gh` 传入;`metrics` 字段供 Task 8 的 Snapshot 测试复用同一 fake。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/ingest/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ingest/discovery.go internal/ingest/discovery_test.go
git commit -m "feat(ingest): discovery job upserts repositories from search slices"
```

---

## Task 8: Snapshot 作业(算 delta、写库)

**Files:**
- Create: `internal/ingest/snapshot.go`
- Modify: `internal/ingest/discovery_test.go`(给 fakeClient 加 FetchByNodeIDs)
- Test: `internal/ingest/snapshot_test.go`

- [ ] **Step 1: 给 fakeClient 补上 FetchByNodeIDs**

在 `internal/ingest/discovery_test.go` 的 `fakeClient` 后追加方法(返回预置的 `github.RepoMetrics`,测试自行保证与请求的 nodeIDs 一致):
```go
func (f *fakeClient) FetchByNodeIDs(_ context.Context, _ []string) ([]github.RepoMetrics, error) {
	return f.metrics, nil
}
```

> `fakeClient` 现同时实现 `Discoverer` 与 `Fetcher`,可作为同一个 fake 注入两个作业。

- [ ] **Step 2: 写失败的 Snapshot 测试**

`internal/ingest/snapshot_test.go`:
```go
package ingest

import (
	"context"
	"testing"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestRunSnapshotComputesDeltaAndWrites(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: 111, NodeID: "R_111", FullName: "a/x", Owner: "a", Name: "x", HTMLURL: "u",
	})
	require.NoError(t, err)

	// 预置昨天的快照:stars=400
	require.NoError(t, db.InsertSnapshot(store.Snapshot{
		RepositoryID: id, Date: "2026-06-02", Stars: 400,
	}))

	fc := &fakeClient{metrics: []github.RepoMetrics{
		{GitHubID: 111, Stars: 500, Forks: 40, OpenIssues: 7, Watchers: 9},
	}}

	err = RunSnapshot(context.Background(), db, fc, "2026-06-03", 100)
	require.NoError(t, err)

	// delta = 500 - 400 = 100
	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 500, stars)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, 500, got.Stars)      // 冗余字段已更新
	require.Equal(t, "2026-06-03T", got.LastSyncedAt[:11]) // 已写同步时间(日期前缀)

	var delta int
	err = db.SQL().QueryRow(
		`SELECT star_delta FROM repository_snapshots WHERE repository_id=? AND snapshot_date=?`,
		id, "2026-06-03").Scan(&delta)
	require.NoError(t, err)
	require.Equal(t, 100, delta)
}

func TestRunSnapshotFirstTimeDeltaIsStars(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: 222, NodeID: "R_222", FullName: "a/y", Owner: "a", Name: "y", HTMLURL: "u",
	})
	require.NoError(t, err)

	fc := &fakeClient{metrics: []github.RepoMetrics{
		{GitHubID: 222, Stars: 50, Forks: 1, OpenIssues: 0, Watchers: 0},
	}}
	require.NoError(t, RunSnapshot(context.Background(), db, fc, "2026-06-03", 100))

	var delta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT star_delta FROM repository_snapshots WHERE repository_id=?`, id).Scan(&delta))
	require.Equal(t, 0, delta) // 无历史快照时 delta 记 0,避免把存量当增量
}
```

> 注意上面用到 `LastSyncedAt[:11]` 断言形如 `2026-06-03T` 的前缀;`RunSnapshot` 写 `last_synced_at` 时用 `date + "T00:00:00Z"`,见实现。

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/ingest/ -run TestRunSnapshot -v`
Expected: FAIL — `undefined: RunSnapshot` / `RepoMetrics`。

- [ ] **Step 4: 实现 Snapshot 作业**

`internal/ingest/snapshot.go`:
```go
package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
)

// Fetcher 是 Snapshot 作业依赖的 GitHub 能力子集(返回 github.RepoMetrics 这一唯一类型)。
type Fetcher interface {
	FetchByNodeIDs(ctx context.Context, nodeIDs []string) ([]github.RepoMetrics, error)
}

// RunSnapshot 给所有活跃仓库拍当日快照:批量拉指标 -> 算 star_delta -> 写快照 + 更新冗余字段。
func RunSnapshot(ctx context.Context, db *store.DB, gh Fetcher, date string, batchSize int) error {
	repos, err := db.ListActiveRepositories()
	if err != nil {
		return err
	}
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 100
	}

	// 建立 github_id -> 仓库内部 id 的映射,并按 node_id 分批。
	idByGitHubID := make(map[int64]int64, len(repos))
	nodeIDs := make([]string, 0, len(repos))
	for _, r := range repos {
		idByGitHubID[r.GitHubID] = r.ID
		nodeIDs = append(nodeIDs, r.NodeID)
	}

	syncedAt := date + "T00:00:00Z"
	written := 0

	for start := 0; start < len(nodeIDs); start += batchSize {
		end := start + batchSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}
		batch := nodeIDs[start:end]

		metrics, err := gh.FetchByNodeIDs(ctx, batch)
		if err != nil {
			return err
		}

		for _, m := range metrics {
			repoID, ok := idByGitHubID[m.GitHubID]
			if !ok {
				continue
			}

			delta := 0
			if prev, has, err := db.LastStars(repoID); err != nil {
				return err
			} else if has {
				delta = m.Stars - prev
			}

			if err := db.InsertSnapshot(store.Snapshot{
				RepositoryID: repoID, Date: date,
				Stars: m.Stars, Forks: m.Forks, OpenIssues: m.OpenIssues, Watchers: m.Watchers,
				StarDelta: delta,
			}); err != nil {
				return err
			}
			if err := db.UpdateRepositoryMetrics(m.GitHubID, m.Stars, m.Forks, m.OpenIssues, m.Watchers, syncedAt); err != nil {
				return err
			}
			written++
		}
	}
	slog.Info("snapshot complete", "date", date, "repos", len(repos), "written", written)
	return nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/ingest/...`
Expected: PASS（discovery 与 snapshot 测试都过）。

- [ ] **Step 6: 提交**

```bash
git add internal/ingest/snapshot.go internal/ingest/snapshot_test.go internal/ingest/discovery_test.go
git commit -m "feat(ingest): daily snapshot job computes star delta and updates repos"
```

---

## Task 9: 调度器与 main 装配

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Create: `cmd/trends/main.go`
- Test: `internal/scheduler/scheduler_test.go`

- [ ] **Step 1: 写失败的调度器测试**

`internal/scheduler/scheduler_test.go`:
```go
package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistersJobsWithoutError(t *testing.T) {
	called := 0
	s, err := New(
		Job{Spec: "@every 1h", Run: func() { called++ }},
		Job{Spec: "@every 2h", Run: func() { called++ }},
	)
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, 2, s.EntryCount())
}

func TestNewRejectsBadSpec(t *testing.T) {
	_, err := New(Job{Spec: "not-a-cron", Run: func() {}})
	require.Error(t, err)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/scheduler/...`
Expected: FAIL — `undefined: New`。

- [ ] **Step 3: 实现调度器**

`internal/scheduler/scheduler.go`:
```go
package scheduler

import "github.com/robfig/cron/v3"

type Job struct {
	Spec string // cron 表达式,如 "0 0 * * *" 或 "@every 1h"
	Run  func()
}

type Scheduler struct {
	cron *cron.Cron
}

// New 构造调度器并注册所有作业;任一 spec 非法则返回错误。
func New(jobs ...Job) (*Scheduler, error) {
	c := cron.New()
	for _, j := range jobs {
		if _, err := c.AddFunc(j.Spec, j.Run); err != nil {
			return nil, err
		}
	}
	return &Scheduler{cron: c}, nil
}

func (s *Scheduler) Start() { s.cron.Start() }

func (s *Scheduler) Stop() { s.cron.Stop() }

func (s *Scheduler) EntryCount() int { return len(s.cron.Entries()) }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/scheduler/...`
Expected: PASS。

- [ ] **Step 5: 写 main 装配(无单测,靠编译 + 手动验证)**

`cmd/trends/main.go`:
```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meirongdev/trends/internal/config"
	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/ingest"
	"github.com/meirongdev/trends/internal/scheduler"
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

	runDiscovery := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := ingest.RunDiscovery(ctx, db, gh, discoveryQueries, 10); err != nil {
			slog.Error("discovery job", "err", err)
		}
	}
	runSnapshot := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		date := time.Now().UTC().Format("2006-01-02")
		if err := ingest.RunSnapshot(ctx, db, gh, date, 100); err != nil {
			slog.Error("snapshot job", "err", err)
		}
	}

	// RUN_ONCE=discovery|snapshot 用于手动触发一次后退出,便于本地验证。
	switch os.Getenv("RUN_ONCE") {
	case "discovery":
		runDiscovery()
		return
	case "snapshot":
		runSnapshot()
		return
	}

	sch, err := scheduler.New(
		scheduler.Job{Spec: cfg.DiscoveryCron, Run: runDiscovery},
		scheduler.Job{Spec: cfg.SnapshotCron, Run: runSnapshot},
	)
	if err != nil {
		slog.Error("scheduler", "err", err)
		os.Exit(1)
	}
	sch.Start()
	defer sch.Stop()
	slog.Info("trends started", "discovery_cron", cfg.DiscoveryCron, "snapshot_cron", cfg.SnapshotCron)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")
}
```

- [ ] **Step 6: 验证整体编译与全部测试**

Run:
```bash
go build ./...
go test ./...
```
Expected: 编译无错;所有包测试 PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/scheduler/ cmd/trends/main.go
git commit -m "feat: cron scheduler and main wiring for ingest jobs"
```

---

## Task 10: Litestream 备份配置与运行文档(运维)

**Files:**
- Create: `litestream.yml`
- Create: `README.md`(运行说明片段)

- [ ] **Step 1: 写 Litestream 配置**

`litestream.yml`(把 `${...}` 在部署环境用真实值替换或经环境变量注入):
```yaml
dbs:
  - path: ${DB_PATH}            # 与 trends 的 DB_PATH 一致,如 /data/trends.db
    replicas:
      - type: s3
        bucket: ${LITESTREAM_BUCKET}
        path: trends
        endpoint: ${LITESTREAM_ENDPOINT}   # 用 R2/MinIO 时设置;AWS S3 可省略
        access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
```

- [ ] **Step 2: 写运行说明**

`README.md`:
```markdown
# Trends — GitHub 趋势仓库追踪平台

详见 [spec.md](./spec.md)。当前实现:**M0 数据地基**(见 docs/superpowers/plans/2026-06-03-m0-data-foundation.md)。

## 本地运行

环境变量:
- `DB_PATH`(默认 `trends.db`)
- `GITHUB_TOKENS`(逗号分隔,可多 token 轮换;留空则不鉴权、额度很低)

构建与测试:
    go build ./...
    go test ./...

手动跑一次作业(便于验证):
    RUN_ONCE=discovery GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends
    RUN_ONCE=snapshot  GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends

常驻(按 cron 调度):
    GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends

## 备份(生产)

用 Litestream 把 SQLite WAL 持续备份到对象存储:
    litestream replicate -config litestream.yml
```

- [ ] **Step 3: 验证 SQL 与配置(手动冒烟)**

Run(需一个真实 GitHub token):
```bash
RUN_ONCE=discovery GITHUB_TOKENS=<your_token> DB_PATH=/tmp/trends.db go run ./cmd/trends
```
Expected: 日志出现 `discovery complete ... upserted=N`(N>0);`/tmp/trends.db` 中 `repositories` 有数据。再跑:
```bash
RUN_ONCE=snapshot GITHUB_TOKENS=<your_token> DB_PATH=/tmp/trends.db go run ./cmd/trends
```
Expected: 日志 `snapshot complete ... written=N`;`repository_snapshots` 有当日行。

- [ ] **Step 4: 提交**

```bash
git add litestream.yml README.md
git commit -m "chore: litestream backup config and run docs"
```

---

## Self-Review(已核对)

- **Spec 覆盖**:本计划对应 spec §3(单二进制+调度)、§4(宇宙发现 + GraphQL 批量 + 限流基础)、§6(`repositories`/`repository_snapshots` 建表)、§11(Litestream)、§14 M0。**评分(§5)、`trending_rankings`、API(§7)、前端(§8)属于 M1/M2,本计划不含**,符合范围拆分。
- **占位符**:无 TBD/TODO;每个代码步骤均给出完整代码与可运行命令。
- **类型一致性**:`store.Repository`/`store.Snapshot` 字段在各任务间一致。`github.RepoMetrics` 是指标的**唯一命名类型**——`ingest.Fetcher` 接口直接返回 `[]github.RepoMetrics`(依赖方向 ingest→github,无循环:github 仅依赖 store)。因此真实 `*github.Client` 的 `SearchRepositories`/`FetchByNodeIDs` 方法签名与 `ingest.Discoverer`/`ingest.Fetcher` 完全一致,可在 main 中直接注入。(注:Go 接口要求返回类型逐字相同,故刻意不在 ingest 另立同字段的 `RepoMetrics`,否则 `*github.Client` 无法满足接口。)
- **已知简化(M1 处理)**:GitHub 限流退避/多 token 配额感知仅做基础(轮换 + 超时),正式退避重试与 `rateLimit` 反馈控速在 M1 强化;宇宙淘汰(`is_active=0`)逻辑在 M1 随评分一并加入。

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-03-m0-data-foundation.md`.
