# 提交收录 OAuth 登录门槛 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 未登录不能提交收录;支持用 GitHub / Google 账户经 OAuth2 登录,引入服务端会话与最小用户存储。

**Architecture:** 新增 `internal/auth`(OAuth provider 配置 + 流程 + token 助手,基于 `golang.org/x/oauth2`)。`internal/store` 加 `users`/`sessions` 表与 `submissions.user_id`,会话以 token 的 sha256 为主键。`internal/api` 加 4 个 auth 端点 + `currentUser` 助手,并把 `POST /submissions` 改为要求登录、按用户限流。前端加 `AuthContext` 与登录态 UI。OAuth 未配置时优雅降级,应用其余功能照常。

**Tech Stack:** Go 1.26(net/http ServeMux + `golang.org/x/oauth2`)、`modernc.org/sqlite`、React + TS + Vite + Tailwind、Vitest + RTL。

**Spec:** `docs/superpowers/specs/2026-06-04-submission-auth-design.md`

**Conventions(本仓库):**
- 离线构建用 `GOPROXY=off`;包路径用 `./cmd/... ./internal/...`(不要 `./...`)。
- 前端:`web/` 下 `npx vitest run`、`npx tsc --noEmit`;`make web` 构建并拷进 `internal/api/dist`。
- e2e 后还原占位:`git checkout -- internal/api/dist/index.html`。
- 提交信息结尾:`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`。

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `internal/store/migrations/0005_auth.sql`(新) | `users`、`sessions` 表;`submissions.user_id` 列 |
| `internal/store/user.go`(新) | `User` 类型、`UpsertUser` |
| `internal/store/session.go`(新) | `CreateSession`、`SessionUser`、`DeleteSession`、`DeleteExpiredSessions` |
| `internal/store/submission.go`(改) | `InsertSubmission` 增加 `userID` 参数 |
| `internal/auth/token.go`(新) | `RandomToken`、`HashToken` |
| `internal/auth/provider.go`(新) | `Identity`、`Provider`、`ProviderSpec`、`NewProvider`、`Github/GoogleParse`、endpoint 常量、`NewProviders`、`Config` |
| `internal/config/config.go`(改) | OAuth 相关 env 字段 |
| `internal/api/auth.go`(新) | login/callback/me/logout 处理器、cookie 助手、`currentUser` |
| `internal/api/server.go`(改) | `NewServer` 签名 + 注册路由 |
| `internal/api/submissions.go`(改) | 要求登录 + 按用户限流 + 记 `user_id` |
| `internal/api/health_test.go`(改) | `newTestServer` 适配新 `NewServer` 签名 |
| `cmd/trends/main.go`(改) | 构建 providers 并传入 `NewServer` |
| `web/src/auth/client.ts`、`AuthContext.tsx`(新) | 前端 auth client + 上下文 |
| `web/src/api/types.ts`(改) | `AuthUser`、`MeResponse` |
| `web/src/App.tsx`、`components/Layout.tsx`、`pages/Submit.tsx`(改) | 包 AuthProvider、导航登录态、提交页门槛 |

依赖方向:`auth → stdlib + x/oauth2`;`store` 不依赖 auth;`api → {store, auth}`;`cmd → all`。

---

## Task 1: 引入 x/oauth2 依赖(前置,需联网)

**Files:** `go.mod`、`go.sum`

> 沙箱挡网络;此步由用户经 `! ` 执行,或在允许禁用沙箱时执行。只取核心包。

- [ ] **Step 1: 拉取依赖**

```bash
cd /Users/matthew/projects/meirongdev/trends
go get golang.org/x/oauth2@latest
go mod tidy
```

- [ ] **Step 2: 验证可离线编译**

Run: `GOPROXY=off go build ./cmd/... ./internal/...`
Expected: 编译通过(此时无新代码,仅确认依赖进入 module cache)。

- [ ] **Step 3: 确认未引入重型传递依赖**

Run: `go list -deps golang.org/x/oauth2 | grep -i 'cloud.google\|appengine' || echo OK`
Expected: `OK`(只用核心 `oauth2`,不 import `/google` 子包)。

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add golang.org/x/oauth2 dependency"
```

---

## Task 2: 迁移 0005 — users / sessions / submissions.user_id

**Files:**
- Create: `internal/store/migrations/0005_auth.sql`
- Test: `internal/store/migration_auth_test.go`

- [ ] **Step 1: 写迁移**

`internal/store/migrations/0005_auth.sql`:

```sql
CREATE TABLE users (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    provider         TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    login            TEXT NOT NULL,
    email            TEXT,
    avatar_url       TEXT,
    created_at       TEXT NOT NULL,
    UNIQUE(provider, provider_user_id)
);

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

ALTER TABLE submissions ADD COLUMN user_id INTEGER REFERENCES users(id);
```

- [ ] **Step 2: 写失败测试**

`internal/store/migration_auth_test.go`:

```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration0005Tables(t *testing.T) {
	db := newTestDB(t)
	for _, tbl := range []string{"users", "sessions"} {
		var name string
		err := db.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		require.NoError(t, err, "table %s should exist", tbl)
		require.Equal(t, tbl, name)
	}
	// submissions.user_id 列存在
	var cnt int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('submissions') WHERE name='user_id'`).Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)
}
```

- [ ] **Step 3: 运行确认失败 → 通过**

Run: `GOPROXY=off go test ./internal/store/ -run TestMigration0005Tables -v`
Expected: 加了迁移后 PASS(若迁移未被 embed 识别会 FAIL)。

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/0005_auth.sql internal/store/migration_auth_test.go
git commit -m "feat(store): add 0005 auth migration (users, sessions, submissions.user_id)"
```

---

## Task 3: store — UpsertUser

**Files:**
- Create: `internal/store/user.go`
- Test: `internal/store/user_test.go`

- [ ] **Step 1: 写失败测试**

`internal/store/user_test.go`:

```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpsertUser(t *testing.T) {
	db := newTestDB(t)
	id1, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "42", Login: "octocat", Email: "o@x.com", AvatarURL: "av1"})
	require.NoError(t, err)
	require.NotZero(t, id1)

	// 同一 (provider, provider_user_id) → 更新而非新建,id 不变
	id2, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "42", Login: "octocat-renamed", Email: "o@x.com", AvatarURL: "av2"})
	require.NoError(t, err)
	require.Equal(t, id1, id2)

	var login, avatar string
	require.NoError(t, db.db.QueryRow(`SELECT login, avatar_url FROM users WHERE id=?`, id1).Scan(&login, &avatar))
	require.Equal(t, "octocat-renamed", login)
	require.Equal(t, "av2", avatar)

	// 不同 provider 同 id → 新用户
	id3, err := db.UpsertUser(User{Provider: "google", ProviderUserID: "42", Login: "alice", AvatarURL: "av3"})
	require.NoError(t, err)
	require.NotEqual(t, id1, id3)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run TestUpsertUser`
Expected: FAIL（`db.UpsertUser undefined` / `User undefined`）。

- [ ] **Step 3: 实现**

`internal/store/user.go`:

```go
package store

// User 是经 OAuth 登录的用户。
type User struct {
	ID             int64
	Provider       string
	ProviderUserID string
	Login          string
	Email          string
	AvatarURL      string
	CreatedAt      string
}

// UpsertUser 按 (provider, provider_user_id) 插入或更新登录/邮箱/头像,返回内部 id。
// created_at 仅插入时设置。
func (d *DB) UpsertUser(u User) (int64, error) {
	_, err := d.db.Exec(`
INSERT INTO users (provider, provider_user_id, login, email, avatar_url, created_at)
VALUES (?,?,?,?,?,?)
ON CONFLICT(provider, provider_user_id) DO UPDATE SET
  login      = excluded.login,
  email      = excluded.email,
  avatar_url = excluded.avatar_url`,
		u.Provider, u.ProviderUserID, u.Login, u.Email, u.AvatarURL, nowUTC())
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.QueryRow(
		`SELECT id FROM users WHERE provider=? AND provider_user_id=?`, u.Provider, u.ProviderUserID,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/store/ -run TestUpsertUser`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/user.go internal/store/user_test.go
git commit -m "feat(store): add UpsertUser"
```

---

## Task 4: store — 会话 CRUD

**Files:**
- Create: `internal/store/session.go`
- Test: `internal/store/session_test.go`

- [ ] **Step 1: 写失败测试**

`internal/store/session_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessions(t *testing.T) {
	db := newTestDB(t)
	uid, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "1", Login: "u", AvatarURL: "a"})
	require.NoError(t, err)

	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	require.NoError(t, db.CreateSession("hash-valid", uid, future))

	u, ok, err := db.SessionUser("hash-valid")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "u", u.Login)
	require.Equal(t, uid, u.ID)

	// 未知 token
	_, ok, err = db.SessionUser("nope")
	require.NoError(t, err)
	require.False(t, ok)

	// 过期会话视为无效
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	require.NoError(t, db.CreateSession("hash-expired", uid, past))
	_, ok, err = db.SessionUser("hash-expired")
	require.NoError(t, err)
	require.False(t, ok)

	// 删除
	require.NoError(t, db.DeleteSession("hash-valid"))
	_, ok, _ = db.SessionUser("hash-valid")
	require.False(t, ok)

	// 清理过期:只删过期行
	n, err := db.DeleteExpiredSessions()
	require.NoError(t, err)
	require.Equal(t, int64(1), n) // hash-expired
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run TestSessions`
Expected: FAIL（方法未定义）。

- [ ] **Step 3: 实现**

`internal/store/session.go`:

```go
package store

// CreateSession 写入一条会话(id 为 token 的 sha256;expiresAt 为 RFC3339)。
func (d *DB) CreateSession(tokenHash string, userID int64, expiresAt string) error {
	_, err := d.db.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?,?,?,?)`,
		tokenHash, userID, nowUTC(), expiresAt)
	return err
}

// SessionUser 按 token 的 sha256 取未过期会话对应的用户;不存在或已过期 → ok=false。
func (d *DB) SessionUser(tokenHash string) (User, bool, error) {
	var u User
	err := d.db.QueryRow(`
SELECT u.id, u.provider, u.provider_user_id, u.login, COALESCE(u.email,''), COALESCE(u.avatar_url,''), u.created_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.id = ? AND s.expires_at > ?`, tokenHash, nowUTC()).
		Scan(&u.ID, &u.Provider, &u.ProviderUserID, &u.Login, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return u, true, nil
}

// DeleteSession 删除指定会话(登出)。
func (d *DB) DeleteSession(tokenHash string) error {
	_, err := d.db.Exec(`DELETE FROM sessions WHERE id=?`, tokenHash)
	return err
}

// DeleteExpiredSessions 删除所有已过期会话,返回删除条数。
func (d *DB) DeleteExpiredSessions() (int64, error) {
	res, err := d.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
```

> 注：`sql: no rows` 比较用字符串以避免在本文件额外 import `database/sql`/`errors`;若该包已 import `errors`,改用 `errors.Is(err, sql.ErrNoRows)` 更佳。本仓库 `store` 包已 import `database/sql`(见 `query.go`),故**改用** `errors.Is(err, sql.ErrNoRows)`:在文件顶部 `import ("database/sql"; "errors")`,把判断替换为 `if errors.Is(err, sql.ErrNoRows) { return User{}, false, nil }`。

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/store/ -run TestSessions`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/session.go internal/store/session_test.go
git commit -m "feat(store): add session CRUD (sha256-keyed, expiry-aware)"
```

---

## Task 5: store — InsertSubmission 关联用户

**Files:**
- Modify: `internal/store/submission.go`
- Test: `internal/store/submission_test.go`(若不存在则创建)

- [ ] **Step 1: 写失败测试**

追加到 `internal/store/submission_test.go`:

```go
func TestInsertSubmissionWithUser(t *testing.T) {
	db := newTestDB(t)
	uid, err := db.UpsertUser(User{Provider: "github", ProviderUserID: "9", Login: "dev", AvatarURL: "a"})
	require.NoError(t, err)

	id, err := db.InsertSubmission("owner/repo", "1.2.3.4", uid)
	require.NoError(t, err)
	require.NotZero(t, id)

	var gotUser int64
	require.NoError(t, db.db.QueryRow(`SELECT user_id FROM submissions WHERE id=?`, id).Scan(&gotUser))
	require.Equal(t, uid, gotUser)
}
```

> 若文件不存在,文件头加 `package store` 与 `import ("testing"; "github.com/stretchr/testify/require")`。

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run TestInsertSubmissionWithUser`
Expected: FAIL（`InsertSubmission` 参数数量不符 → 编译错误）。

- [ ] **Step 3: 改实现**

`internal/store/submission.go` 中 `InsertSubmission`:

```go
// InsertSubmission 记录一条 pending 提交(关联登录用户),返回 id。
func (d *DB) InsertSubmission(fullName, ip string, userID int64) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO submissions (full_name, status, submitted_ip, user_id, created_at) VALUES (?, 'pending', ?, ?, ?)`,
		fullName, ip, userID, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
```

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/store/ -run TestInsertSubmissionWithUser`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/submission.go internal/store/submission_test.go
git commit -m "feat(store): link submissions to user_id"
```

---

## Task 6: auth — token 助手

**Files:**
- Create: `internal/auth/token.go`
- Test: `internal/auth/token_test.go`

- [ ] **Step 1: 写失败测试**

`internal/auth/token_test.go`:

```go
package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomTokenAndHash(t *testing.T) {
	a, err := RandomToken(32)
	require.NoError(t, err)
	b, err := RandomToken(32)
	require.NoError(t, err)
	require.NotEqual(t, a, b)       // 随机
	require.GreaterOrEqual(t, len(a), 32)

	h1 := HashToken(a)
	require.Equal(t, h1, HashToken(a)) // 确定性
	require.NotEqual(t, a, h1)          // 哈希后不等于原值
	require.Len(t, h1, 64)              // sha256 hex
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/auth/ -run TestRandomTokenAndHash`
Expected: FAIL（包/函数未定义）。

- [ ] **Step 3: 实现**

`internal/auth/token.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// RandomToken 返回 nBytes 字节随机数据的 base64url(无填充)编码。
func RandomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken 返回 raw 的 sha256 十六进制(用于会话主键,DB 不存原始 token)。
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/auth/ -run TestRandomTokenAndHash`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/token.go internal/auth/token_test.go
git commit -m "feat(auth): add random token + sha256 hash helpers"
```

---

## Task 7: auth — Provider 与流程

**Files:**
- Create: `internal/auth/provider.go`
- Test: `internal/auth/provider_test.go`

- [ ] **Step 1: 写失败测试**

`internal/auth/provider_test.go`:

```go
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

	// AuthCodeURL 含必要参数
	u := p.AuthCodeURL("state123")
	require.True(t, strings.HasPrefix(u, srv.URL+"/authorize"))
	require.Contains(t, u, "client_id=cid")
	require.Contains(t, u, "state=state123")

	// Exchange + FetchIdentity
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
		// Google 未配置
	})
	require.Contains(t, ps, "github")
	require.NotContains(t, ps, "google")
	require.Equal(t, "http://localhost:8080/api/v1/auth/callback?provider=github", ps["github"].RedirectURL())
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/auth/ -run 'TestGithubParse|TestGoogleParse|TestProviderFlow|TestNewProvidersOnlyConfigured'`
Expected: FAIL（未定义)。

- [ ] **Step 3: 实现**

`internal/auth/provider.go`:

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
)

// Identity 是从 provider 取回并归一后的用户身份。
type Identity struct {
	Provider       string
	ProviderUserID string
	Login          string
	Email          string
	AvatarURL      string
}

// Provider 的真实/测试 endpoint 与解析函数。
const (
	GithubAuthURL     = "https://github.com/login/oauth/authorize"
	GithubTokenURL    = "https://github.com/login/oauth/access_token"
	GithubUserInfoURL = "https://api.github.com/user"
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// ProviderSpec 描述一个可构造的 provider(endpoint 可注入以便测试)。
type ProviderSpec struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	Parse        func([]byte) (Identity, error)
}

// Provider 封装一个 OAuth2 provider 的授权/换 token/取身份。
type Provider struct {
	name        string
	oauth       *oauth2.Config
	userInfoURL string
	parse       func([]byte) (Identity, error)
	client      *http.Client
}

func NewProvider(s ProviderSpec) *Provider {
	scopes := s.Scopes
	return &Provider{
		name: s.Name,
		oauth: &oauth2.Config{
			ClientID:     s.ClientID,
			ClientSecret: s.ClientSecret,
			RedirectURL:  s.RedirectURL,
			Scopes:       scopes,
			Endpoint:     oauth2.Endpoint{AuthURL: s.AuthURL, TokenURL: s.TokenURL},
		},
		userInfoURL: s.UserInfoURL,
		parse:       s.Parse,
		client:      http.DefaultClient,
	}
}

func (p *Provider) Name() string        { return p.name }
func (p *Provider) RedirectURL() string { return p.oauth.RedirectURL }

func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauth.Exchange(ctx, code)
}

// FetchIdentity 用 access token 调 userinfo,归一为 Identity。
func (p *Provider) FetchIdentity(ctx context.Context, tok *oauth2.Token) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("userinfo %s: status %d", p.name, resp.StatusCode)
	}
	return p.parse(body)
}

// GithubParse 解析 GitHub /user 响应。
func GithubParse(body []byte) (Identity, error) {
	var g struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		return Identity{}, err
	}
	return Identity{
		Provider: "github", ProviderUserID: strconv.FormatInt(g.ID, 10),
		Login: g.Login, Email: g.Email, AvatarURL: g.AvatarURL,
	}, nil
}

// GoogleParse 解析 Google OIDC userinfo 响应。
func GoogleParse(body []byte) (Identity, error) {
	var g struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		return Identity{}, err
	}
	login := g.Name
	if login == "" {
		login = g.Email
	}
	return Identity{
		Provider: "google", ProviderUserID: g.Sub,
		Login: login, Email: g.Email, AvatarURL: g.Picture,
	}, nil
}

// Config 是从应用配置构造 providers 所需的输入。
type Config struct {
	BaseURL            string
	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
}

// NewProviders 仅为配置齐全(id+secret 都有)的 provider 构造实例。
func NewProviders(c Config) map[string]*Provider {
	out := map[string]*Provider{}
	if c.GitHubClientID != "" && c.GitHubClientSecret != "" {
		out["github"] = NewProvider(ProviderSpec{
			Name: "github", ClientID: c.GitHubClientID, ClientSecret: c.GitHubClientSecret,
			RedirectURL: c.BaseURL + "/api/v1/auth/callback?provider=github",
			AuthURL:     GithubAuthURL, TokenURL: GithubTokenURL, UserInfoURL: GithubUserInfoURL,
			Scopes: []string{"read:user"}, Parse: GithubParse,
		})
	}
	if c.GoogleClientID != "" && c.GoogleClientSecret != "" {
		out["google"] = NewProvider(ProviderSpec{
			Name: "google", ClientID: c.GoogleClientID, ClientSecret: c.GoogleClientSecret,
			RedirectURL: c.BaseURL + "/api/v1/auth/callback?provider=google",
			AuthURL:     GoogleAuthURL, TokenURL: GoogleTokenURL, UserInfoURL: GoogleUserInfoURL,
			Scopes: []string{"openid", "email", "profile"}, Parse: GoogleParse,
		})
	}
	return out
}
```

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/auth/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/provider.go internal/auth/provider_test.go
git commit -m "feat(auth): add OAuth provider (github/google), flow + identity parse"
```

---

## Task 8: config — OAuth env 字段

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`(若不存在则创建)

- [ ] **Step 1: 写失败测试**

追加/创建 `internal/config/config_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
```

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/config/ -run TestLoadOAuth`
Expected: FAIL（字段未定义）。

- [ ] **Step 3: 实现**

`internal/config/config.go` —— `Config` 结构体追加字段:

```go
	OAuthBaseURL            string
	GitHubOAuthClientID     string
	GitHubOAuthClientSecret string
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
```

`Load()` 的返回字面量追加:

```go
		OAuthBaseURL:            getenv("OAUTH_BASE_URL", "http://localhost:8080"),
		GitHubOAuthClientID:     os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubOAuthClientSecret: os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		GoogleOAuthClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
```

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/config/ -run TestLoadOAuth`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add OAuth client + base URL env"
```

---

## Task 9: api — Server 接线 + cookie/currentUser 助手

**Files:**
- Modify: `internal/api/server.go`
- Create: `internal/api/auth.go`(仅 cookie 助手 + currentUser;handlers 在 Task 10)
- Modify: `internal/api/health_test.go`(`newTestServer` 适配)
- Modify: `cmd/trends/main.go`(临时:本任务先改 NewServer 调用,避免编译断裂)

- [ ] **Step 1: 改 `Server` 与 `NewServer`**

`internal/api/server.go` —— 顶部 import 加 `"strings"`(已存在 `strconv`/`time`)与 `"github.com/meirongdev/trends/internal/auth"`。改结构体与构造器:

```go
type Server struct {
	db            *store.DB
	submitLimiter *rateLimiter
	providers     map[string]*auth.Provider
	baseURL       string
	secureCookies bool
}

func NewServer(db *store.DB, providers map[string]*auth.Provider, baseURL string) *Server {
	return &Server{
		db:            db,
		submitLimiter: newRateLimiter(30, 24*time.Hour), // 每用户 30 次/天
		providers:     providers,
		baseURL:       baseURL,
		secureCookies: strings.HasPrefix(baseURL, "https"),
	}
}
```

- [ ] **Step 2: 写 cookie 助手 + currentUser**

`internal/api/auth.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/meirongdev/trends/internal/auth"
	"github.com/meirongdev/trends/internal/store"
)

const (
	sessionCookieName = "trends_session"
	stateCookieName   = "trends_oauth_state"
	sessionTTL        = 30 * 24 * time.Hour
	stateTTL          = 10 * time.Minute
)

func (s *Server) setCookie(w http.ResponseWriter, name, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", HttpOnly: true,
		Secure: s.secureCookies, SameSite: http.SameSiteLaxMode,
		MaxAge: int(maxAge.Seconds()),
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true,
		Secure: s.secureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// currentUser 从 session cookie 解析当前登录用户。
func (s *Server) currentUser(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return store.User{}, false
	}
	u, ok, err := s.db.SessionUser(auth.HashToken(c.Value))
	if err != nil || !ok {
		return store.User{}, false
	}
	return u, true
}
```

- [ ] **Step 3: 适配 `newTestServer`**

`internal/api/health_test.go` —— 找到 `func newTestServer`,把 `NewServer(db)` 改为:

```go
	s := NewServer(db, map[string]*auth.Provider{}, "")
```

并在该测试文件 import 块加入 `"github.com/meirongdev/trends/internal/auth"`。

- [ ] **Step 4: 适配 `cmd/trends/main.go`(临时最小改动)**

把 `api.NewServer(db)` 改为(完整接线在 Task 12,这里先让它编译):

```go
	server := api.NewServer(db, auth.NewProviders(auth.Config{
		BaseURL:            cfg.OAuthBaseURL,
		GitHubClientID:     cfg.GitHubOAuthClientID,
		GitHubClientSecret: cfg.GitHubOAuthClientSecret,
		GoogleClientID:     cfg.GoogleOAuthClientID,
		GoogleClientSecret: cfg.GoogleOAuthClientSecret,
	}), cfg.OAuthBaseURL)
```

`cmd/trends/main.go` import 加 `"github.com/meirongdev/trends/internal/auth"`。

- [ ] **Step 5: 编译 + 现有 api 测试仍绿**

Run: `GOPROXY=off go build ./cmd/... ./internal/... && GOPROXY=off go test ./internal/api/`
Expected: 编译通过;现有 api 测试 PASS(签名变更未破坏行为)。

- [ ] **Step 6: Commit**

```bash
git add internal/api/server.go internal/api/auth.go internal/api/health_test.go cmd/trends/main.go
git commit -m "feat(api): thread OAuth providers into Server; add cookie + currentUser helpers"
```

---

## Task 10: api — auth 端点(login/callback/me/logout)

**Files:**
- Modify: `internal/api/auth.go`(加 handlers)
- Modify: `internal/api/server.go`(注册路由)
- Test: `internal/api/auth_test.go`

- [ ] **Step 1: 写失败测试**

`internal/api/auth_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meirongdev/trends/internal/auth"
	"github.com/stretchr/testify/require"
)

// fakeProviderServer 返回一个假 OAuth provider(/token + /userinfo)。
func fakeProviderServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "bearer"})
		case "/userinfo":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":55,"login":"alice","avatar_url":"av"}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func serverWithGithub(t *testing.T, providerURL string) (*Server, *store.DB) {
	s, db := newTestServer(t)
	s.providers = map[string]*auth.Provider{
		"github": auth.NewProvider(auth.ProviderSpec{
			Name: "github", ClientID: "cid", ClientSecret: "sec",
			RedirectURL: "http://app/api/v1/auth/callback?provider=github",
			AuthURL:     providerURL + "/authorize", TokenURL: providerURL + "/token",
			UserInfoURL: providerURL + "/userinfo", Parse: auth.GithubParse,
		}),
	}
	return s, db
}

func TestAuthLoginRedirects(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, _ := serverWithGithub(t, prov.URL)

	rec := doGET(t, s, "/api/v1/auth/login?provider=github")
	require.Equal(t, http.StatusFound, rec.Code)
	loc := rec.Header().Get("Location")
	require.True(t, strings.HasPrefix(loc, prov.URL+"/authorize"))
	require.Contains(t, loc, "state=")
	require.Contains(t, rec.Header().Get("Set-Cookie"), stateCookieName)
}

func TestAuthLoginUnconfigured(t *testing.T) {
	s, _ := newTestServer(t) // 无 providers
	rec := doGET(t, s, "/api/v1/auth/login?provider=github")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAuthCallbackHappyPath(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, db := serverWithGithub(t, prov.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?provider=github&code=c&state=st", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "st"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/submit", rec.Header().Get("Location"))
	require.Contains(t, rec.Header().Get("Set-Cookie"), sessionCookieName)

	var n int
	require.NoError(t, db.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE provider='github' AND provider_user_id='55'`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestAuthCallbackBadState(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, _ := serverWithGithub(t, prov.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?provider=github&code=c&state=evil", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "st"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthMeAndLogout(t *testing.T) {
	prov := fakeProviderServer(t)
	defer prov.Close()
	s, db := serverWithGithub(t, prov.URL)

	// 先建用户与会话,拿到原始 token
	uid, err := db.UpsertUser(store.User{Provider: "github", ProviderUserID: "55", Login: "alice", AvatarURL: "av"})
	require.NoError(t, err)
	raw, err := auth.RandomToken(32)
	require.NoError(t, err)
	require.NoError(t, db.CreateSession(auth.HashToken(raw), uid, "2999-01-01T00:00:00Z"))

	// /me 带 cookie
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: raw})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var me struct {
		User      *struct{ Login string } `json:"user"`
		Providers []string                `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &me))
	require.NotNil(t, me.User)
	require.Equal(t, "alice", me.User.Login)
	require.Contains(t, me.Providers, "github")

	// logout
	lreq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	lreq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: raw})
	lrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec, lreq)
	require.Equal(t, http.StatusNoContent, lrec.Code)

	_, ok, _ := db.SessionUser(auth.HashToken(raw))
	require.False(t, ok) // 会话已删
}

func TestAuthMeAnonymous(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/auth/me")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"user":null`)
}
```

> 该测试用到 `db.DB()`(返回底层 `*sql.DB`)。若 `store.DB` 未暴露,本任务 Step 3 顺带添加。

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run TestAuth`
Expected: FAIL（handlers/路由未定义)。

- [ ] **Step 3: 实现 handlers + 路由(+ store.DB() 访问器)**

若 `internal/store/store.go` 未暴露底层句柄,添加:

```go
// DB 返回底层 *sql.DB(仅供测试断言用)。
func (d *DB) DB() *sql.DB { return d.db }
```

`internal/api/auth.go` 追加(import 加 `"encoding/json"`、`"sort"`、`"log/slog"`):

```go
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providers[r.URL.Query().Get("provider")]
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "login not configured for this provider")
		return
	}
	state, err := auth.RandomToken(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.setCookie(w, stateCookieName, state, stateTTL)
	http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, ok := s.providers[q.Get("provider")]
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "login not configured for this provider")
		return
	}
	c, err := r.Cookie(stateCookieName)
	if err != nil || c.Value == "" || c.Value != q.Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	s.clearCookie(w, stateCookieName)

	tok, err := p.Exchange(r.Context(), q.Get("code"))
	if err != nil {
		slog.Warn("oauth exchange failed", "provider", p.Name(), "err", err)
		http.Redirect(w, r, "/submit?auth_error=1", http.StatusFound)
		return
	}
	id, err := p.FetchIdentity(r.Context(), tok)
	if err != nil {
		slog.Warn("oauth identity fetch failed", "provider", p.Name(), "err", err)
		http.Redirect(w, r, "/submit?auth_error=1", http.StatusFound)
		return
	}

	uid, err := s.db.UpsertUser(store.User{
		Provider: id.Provider, ProviderUserID: id.ProviderUserID,
		Login: id.Login, Email: id.Email, AvatarURL: id.AvatarURL,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	raw, err := auth.RandomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	expires := time.Now().UTC().Add(sessionTTL).Format(time.RFC3339)
	if err := s.db.CreateSession(auth.HashToken(raw), uid, expires); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	_, _ = s.db.DeleteExpiredSessions()
	s.setCookie(w, sessionCookieName, raw, sessionTTL)
	http.Redirect(w, r, "/submit", http.StatusFound)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	names := make([]string, 0, len(s.providers))
	for n := range s.providers {
		names = append(names, n)
	}
	sort.Strings(names)

	resp := map[string]any{"user": nil, "providers": names}
	if u, ok := s.currentUser(r); ok {
		resp["user"] = map[string]string{"login": u.Login, "avatar_url": u.AvatarURL, "provider": u.Provider}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		_ = s.db.DeleteSession(auth.HashToken(c.Value))
	}
	s.clearCookie(w, sessionCookieName)
	w.WriteHeader(http.StatusNoContent)
}
```

`internal/api/server.go` 的 `Routes()` 在 submissions 行附近注册:

```go
	mux.HandleFunc("GET /api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("GET /api/v1/auth/callback", s.handleAuthCallback)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleAuthMe)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuthLogout)
```

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/api/ -run TestAuth`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/auth.go internal/api/server.go internal/api/auth_test.go internal/store/store.go
git commit -m "feat(api): add OAuth login/callback/me/logout endpoints"
```

---

## Task 11: api — 提交需登录 + 按用户限流

**Files:**
- Modify: `internal/api/submissions.go`
- Test: `internal/api/submissions_test.go`(追加)

- [ ] **Step 1: 写失败测试**

追加到 `internal/api/submissions_test.go`(import 需含 `net/http/httptest`、`auth`、`store`、`strconv`、`time`):

```go
func loggedInCookie(t *testing.T, db *store.DB) *http.Cookie {
	uid, err := db.UpsertUser(store.User{Provider: "github", ProviderUserID: "1", Login: "u", AvatarURL: "a"})
	require.NoError(t, err)
	raw, err := auth.RandomToken(32)
	require.NoError(t, err)
	require.NoError(t, db.CreateSession(auth.HashToken(raw), uid, "2999-01-01T00:00:00Z"))
	return &http.Cookie{Name: sessionCookieName, Value: raw}
}

func TestSubmitRequiresLogin(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", strings.NewReader(`{"full_name":"o/r"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSubmitLoggedIn(t *testing.T) {
	s, db := newTestServer(t)
	cookie := loggedInCookie(t, db)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", strings.NewReader(`{"full_name":"owner/repo"}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	var uid int64
	require.NoError(t, db.DB().QueryRow(`SELECT user_id FROM submissions WHERE full_name='owner/repo'`).Scan(&uid))
	require.NotZero(t, uid)
}

func TestSubmitRateLimitPerUser(t *testing.T) {
	s, db := newTestServer(t)
	s.submitLimiter = newRateLimiter(1, time.Hour) // 收紧便于断言
	cookie := loggedInCookie(t, db)
	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", strings.NewReader(`{"full_name":"o/r"}`))
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		return rec.Code
	}
	require.Equal(t, http.StatusAccepted, do())
	require.Equal(t, http.StatusTooManyRequests, do()) // 第二次超限
}
```

> 若 `submissions_test.go` 尚未 import 这些包,补齐:`"net/http"`、`"net/http/httptest"`、`"strings"`、`"time"`、`"github.com/meirongdev/trends/internal/auth"`、`"github.com/meirongdev/trends/internal/store"`。

- [ ] **Step 2: 运行确认失败**

Run: `GOPROXY=off go test ./internal/api/ -run TestSubmit`
Expected: FAIL（当前 handler 不要求登录,`TestSubmitRequiresLogin` 不会得到 401）。

- [ ] **Step 3: 改 `handleSubmit`**

`internal/api/submissions.go`(import 加 `"strconv"`;`clientIP`/`fullNameRe` 已在文件中):

```go
// handleSubmit 接收 owner/repo 收录提交:要求登录 + 每用户限流 + 格式校验 + 落库 pending。
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required")
		return
	}
	if !s.submitLimiter.allow(strconv.FormatInt(u.ID, 10)) {
		writeError(w, http.StatusTooManyRequests, "too many submissions, try again later")
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

	id, err := s.db.InsertSubmission(body.FullName, clientIP(r), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "pending"})
}
```

> 删除原 `handleSubmit` 顶部基于 IP 的 `s.submitLimiter.allow(clientIP(r))` 判断(已被每用户限流取代)。

- [ ] **Step 4: 运行确认通过**

Run: `GOPROXY=off go test ./internal/api/ -run TestSubmit`
Expected: PASS

- [ ] **Step 5: 全 Go 套件 + vet**

Run: `GOPROXY=off go test ./cmd/... ./internal/... && GOPROXY=off go vet ./cmd/... ./internal/...`
Expected: 全 PASS / vet 干净。

- [ ] **Step 6: Commit**

```bash
git add internal/api/submissions.go internal/api/submissions_test.go
git commit -m "feat(api): require login for submissions; per-user rate limit"
```

---

## Task 12: cmd/trends — 接线确认

**Files:** `cmd/trends/main.go`(Task 9 已临时接好,这里确认无遗漏)

- [ ] **Step 1: 确认接线**

确认 `cmd/trends/main.go` 中 `api.NewServer(...)` 已使用 `auth.NewProviders(auth.Config{...})` 与 `cfg.OAuthBaseURL`(见 Task 9 Step 4)。无需额外改动则跳过提交。

- [ ] **Step 2: 构建确认**

Run: `GOPROXY=off go build -o /tmp/trends-auth ./cmd/trends`
Expected: 成功。

---

## Task 13: 前端 — auth client + 类型 + AuthContext

**Files:**
- Modify: `web/src/api/types.ts`
- Create: `web/src/auth/client.ts`、`web/src/auth/AuthContext.tsx`
- Test: `web/src/auth/AuthContext.test.tsx`

- [ ] **Step 1: 类型**

`web/src/api/types.ts` 追加:

```ts
export interface AuthUser {
  login: string
  avatar_url: string
  provider: string
}

export interface MeResponse {
  user: AuthUser | null
  providers: string[]
}
```

- [ ] **Step 2: client**

`web/src/auth/client.ts`:

```ts
import type { MeResponse } from '../api/types'

const BASE = '/api/v1/auth'

export async function getMe(): Promise<MeResponse> {
  const res = await fetch(`${BASE}/me`, { credentials: 'same-origin' })
  if (!res.ok) return { user: null, providers: [] }
  return (await res.json()) as MeResponse
}

export function login(provider: string): void {
  window.location.href = `${BASE}/login?provider=${encodeURIComponent(provider)}`
}

export async function logout(): Promise<void> {
  await fetch(`${BASE}/logout`, { method: 'POST', credentials: 'same-origin' })
}
```

- [ ] **Step 3: 写失败测试**

`web/src/auth/AuthContext.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthProvider, useAuth } from './AuthContext'
import * as client from './client'

function Probe() {
  const { user, loading } = useAuth()
  if (loading) return <span>loading</span>
  return <span>{user ? `hi ${user.login}` : 'anon'}</span>
}

describe('AuthContext', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getMe').mockResolvedValue({
      user: { login: 'octocat', avatar_url: 'av', provider: 'github' },
      providers: ['github', 'google'],
    })
  })

  it('exposes the current user after loading', async () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    )
    expect(await screen.findByText('hi octocat')).toBeInTheDocument()
  })
})
```

- [ ] **Step 4: 运行确认失败**

Run: `cd web && npx vitest run src/auth/AuthContext.test.tsx`
Expected: FAIL（模块不存在)。

- [ ] **Step 5: 实现 AuthContext**

`web/src/auth/AuthContext.tsx`:

```tsx
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import type { AuthUser } from '../api/types'
import { getMe, logout as apiLogout } from './client'

interface AuthState {
  user: AuthUser | null
  providers: string[]
  loading: boolean
  refresh: () => void
  logout: () => Promise<void>
}

const Ctx = createContext<AuthState | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [providers, setProviders] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    getMe()
      .then((m) => {
        if (!cancelled) {
          setUser(m.user)
          setProviders(m.providers)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [tick])

  const value: AuthState = {
    user,
    providers,
    loading,
    refresh: () => setTick((t) => t + 1),
    logout: async () => {
      await apiLogout()
      setTick((t) => t + 1)
    },
  }
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth(): AuthState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth must be used within AuthProvider')
  return v
}
```

- [ ] **Step 6: 运行确认通过**

Run: `cd web && npx vitest run src/auth/AuthContext.test.tsx`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/src/api/types.ts web/src/auth/client.ts web/src/auth/AuthContext.tsx web/src/auth/AuthContext.test.tsx
git commit -m "feat(web): add auth client + AuthContext"
```

---

## Task 14: 前端 — 接 AuthProvider、导航登录态、Submit 门槛

**Files:**
- Modify: `web/src/App.tsx`、`web/src/components/Layout.tsx`、`web/src/pages/Submit.tsx`
- Test: `web/src/pages/Submit.test.tsx`(追加/改)

- [ ] **Step 1: App 包 AuthProvider**

`web/src/App.tsx` —— import `import { AuthProvider } from './auth/AuthContext'`,把 `<BrowserRouter>...</BrowserRouter>` 整体包进 `<AuthProvider>`:

```tsx
  return (
    <AuthProvider>
      <BrowserRouter>
        {/* ...原有 Routes 不变... */}
      </BrowserRouter>
    </AuthProvider>
  )
```

- [ ] **Step 2: Layout 显示登录态**

`web/src/components/Layout.tsx` —— import `import { useAuth } from '../auth/AuthContext'`,在组件内取 `const { user, logout } = useAuth()`,在导航「提交」之后加:

```tsx
          {user ? (
            <span className="flex items-center gap-2 text-sm">
              {user.avatar_url && <img src={user.avatar_url} alt="" className="h-6 w-6 rounded-full" />}
              <span className="text-slate-600">{user.login}</span>
              <button onClick={() => logout()} className="text-blue-700 hover:underline">
                登出
              </button>
            </span>
          ) : null}
```

- [ ] **Step 3: 写 Submit 门槛失败测试**

改写 `web/src/pages/Submit.test.tsx`(用 AuthProvider 包裹并 mock getMe):

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AuthProvider } from '../auth/AuthContext'
import { Submit } from './Submit'
import * as authClient from '../auth/client'

function renderSubmit() {
  return render(
    <AuthProvider>
      <MemoryRouter>
        <Submit />
      </MemoryRouter>
    </AuthProvider>,
  )
}

describe('Submit page', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('shows login buttons when logged out', async () => {
    vi.spyOn(authClient, 'getMe').mockResolvedValue({ user: null, providers: ['github', 'google'] })
    renderSubmit()
    expect(await screen.findByRole('button', { name: /GitHub/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Google/i })).toBeInTheDocument()
  })

  it('shows the submit form when logged in', async () => {
    vi.spyOn(authClient, 'getMe').mockResolvedValue({
      user: { login: 'octocat', avatar_url: 'av', provider: 'github' },
      providers: ['github'],
    })
    renderSubmit()
    expect(await screen.findByPlaceholderText('owner/repo')).toBeInTheDocument()
  })
})
```

- [ ] **Step 4: 运行确认失败**

Run: `cd web && npx vitest run src/pages/Submit.test.tsx`
Expected: FAIL（当前 Submit 不感知登录态)。

- [ ] **Step 5: 改 Submit 加门槛**

`web/src/pages/Submit.tsx` 顶部 import 加 `import { useAuth } from '../auth/AuthContext'` 与 `import { login } from '../auth/client'`。在组件内最前面取 `const { user, providers, loading } = useAuth()`,并在 `return` 前分支:

```tsx
  const labels: Record<string, string> = { github: '用 GitHub 登录', google: '用 Google 登录' }

  if (loading) {
    return <p className="text-slate-500">加载中…</p>
  }
  if (!user) {
    return (
      <div className="max-w-lg space-y-4">
        <h1 className="text-xl font-bold">提交收录</h1>
        <p className="text-sm text-slate-500">提交前请先登录。</p>
        <div className="flex gap-2">
          {providers.map((p) => (
            <button
              key={p}
              onClick={() => login(p)}
              className="rounded bg-slate-900 px-3 py-1.5 text-sm text-white"
            >
              {labels[p] ?? p}
            </button>
          ))}
          {providers.length === 0 && <span className="text-sm text-slate-400">登录暂未开放。</span>}
        </div>
      </div>
    )
  }
```

(已登录时返回原有表单不变。)若提交收到 401,在 `catch` 里把 `message` 设为「登录已失效,请重新登录。」即可——在现有 `catch` 分支后追加判断:`if (err instanceof Error && /401|login required/i.test(err.message)) setMessage('登录已失效,请重新登录。')`。

- [ ] **Step 6: 运行确认通过**

Run: `cd web && npx vitest run src/pages/Submit.test.tsx`
Expected: PASS

- [ ] **Step 7: 全前端套件 + 类型检查**

Run: `cd web && npx vitest run && npx tsc --noEmit`
Expected: 全 PASS / tsc 干净。

- [ ] **Step 8: Commit**

```bash
git add web/src/App.tsx web/src/components/Layout.tsx web/src/pages/Submit.tsx web/src/pages/Submit.test.tsx
git commit -m "feat(web): gate Submit behind login; show auth state in nav"
```

---

## Task 15: e2e 冒烟(未配置模式)+ 收尾

**Files:** 无新增;构建 + 运行验证。

- [ ] **Step 1: 构建前端 + 二进制**

```bash
cd /Users/matthew/projects/meirongdev/trends
make web
GOPROXY=off go build -o /tmp/trends-auth ./cmd/trends
git checkout -- internal/api/dist/index.html
```

- [ ] **Step 2: 起服务(不配 OAuth env)并验证降级**

在你的 shell 用 `! ` 运行(或允许时直接跑):

```bash
DB_PATH="$(mktemp -d)/t.db" API_LISTEN_ADDR=127.0.0.1:18140 /tmp/trends-auth >/tmp/trends-auth.log 2>&1 &
sleep 2
curl -s -o /dev/null -w "me=%{http_code}\n"       http://127.0.0.1:18140/api/v1/auth/me
curl -s                                            http://127.0.0.1:18140/api/v1/auth/me; echo
curl -s -o /dev/null -w "login=%{http_code}\n"     "http://127.0.0.1:18140/api/v1/auth/login?provider=github"
curl -s -o /dev/null -w "submit=%{http_code}\n" -X POST -H 'Content-Type: application/json' -d '{"full_name":"o/r"}' http://127.0.0.1:18140/api/v1/submissions
curl -s -o /dev/null -w "spa=%{http_code}\n"       http://127.0.0.1:18140/submit
pkill -f /tmp/trends-auth
```

Expected:
- `me=200`,body 含 `"user":null` 且 `"providers":[]`
- `login=503`(未配置)
- `submit=401`(未登录)
- `spa=200`

- [ ] **Step 3: 全套件复核**

Run: `GOPROXY=off go test ./cmd/... ./internal/... && cd web && npx vitest run`
Expected: 全 PASS。

- [ ] **Step 4: 文档**

更新 `AGENTS.md`:端点数 14 → 18(新增 login/callback/me/logout);提交需登录、按用户限流;新增 env(`OAUTH_BASE_URL`、`GITHUB_OAUTH_CLIENT_ID/SECRET`、`GOOGLE_OAUTH_CLIENT_ID/SECRET`)。

```bash
git add AGENTS.md
git commit -m "docs: update AGENTS.md for submission OAuth login"
```

- [ ] **Step 5: 收尾**

使用 superpowers:finishing-a-development-branch:验证测试 → 呈现合并选项。

---

## Self-Review

**1. Spec 覆盖**
- 数据模型(users/sessions/submissions.user_id)→ Task 2。✓
- token 存 sha256 → Task 6(HashToken)+ Task 4(以 hash 为主键)+ Task 9(currentUser 哈希后查)。✓
- UpsertUser/会话 CRUD → Task 3/4。✓
- x/oauth2 + 自定义 endpoint + github/google parse → Task 1/7。✓
- 4 个 auth 端点 + 流程(state CSRF、Exchange、FetchIdentity、UpsertUser、CreateSession、回 /submit、失败回 /submit?auth_error=1)→ Task 10。✓
- 提交要求登录 + 按用户限流(30/天)+ 记 user_id → Task 11。✓
- 配置 env + 未配置降级(/login 503、/me 匿名)→ Task 8/10/15。✓
- 前端 AuthContext/client、导航登录态、Submit 门槛(含未配置/已配置 provider 按钮)→ Task 13/14。✓
- 安全:httpOnly + SameSite=Lax + https→Secure、state 比对、logout POST、过期清理 → Task 9/10。✓
- 测试覆盖(auth/store/api/前端)→ 各任务含 TDD。✓
- 依赖前置与 OAuth App 注册 → Task 1 + spec「依赖与前置」。✓

**2. Placeholder 扫描:** 无 TBD/TODO;每个改代码步骤均含完整代码。✓

**3. 类型一致性:**
- `store.User{ID,Provider,ProviderUserID,Login,Email,AvatarURL,CreatedAt}` 全任务一致。✓
- `InsertSubmission(fullName, ip string, userID int64)` 在 Task 5 定义、Task 11 调用一致。✓
- `auth.Identity{Provider,ProviderUserID,Login,Email,AvatarURL}`、`auth.ProviderSpec`、`auth.NewProvider`、`auth.HashToken`、`auth.RandomToken`、`auth.NewProviders/Config` 在 Task 6/7 定义,Task 9/10/11/测试中一致引用。✓
- `Provider.Name()`/`RedirectURL()`/`AuthCodeURL()`/`Exchange()`/`FetchIdentity()` 定义于 Task 7,Task 10 使用一致。✓
- `store.DB()` 访问器在 Task 10 引入,供 api/store 测试断言。✓
- cookie 常量 `sessionCookieName`/`stateCookieName`、TTL 在 Task 9 定义,Task 10/11/测试一致。✓
- `NewServer(db, providers, baseURL)` 新签名在 Task 9 定义,`newTestServer`/`cmd/main` 同步更新。✓

## Execution Handoff
Plan complete and saved to `docs/superpowers/plans/2026-06-04-submission-auth.md`.
