# 提交收录 OAuth 登录门槛 — 设计

> 状态:已批准(2026-06-04)。下一步:writing-plans。

## 目标

给提交收录加登录门槛:**未登录不能提交**。支持用 **GitHub** 或 **Google** 账户登录(OAuth2 授权码流程),引入会话与最小用户存储,并与现有的 `pending` + 限流流程整合。

**范围(YAGNI):** 仅门槛提交。不做个人主页 / 「我的提交」列表。数据模型与端点为后续 accounts(Phase 3)预留扩展空间,但本次不实现。

## 关键决策(已确认)

- **OAuth 实现:** `golang.org/x/oauth2`(只 import 核心 `oauth2` 包;GitHub/Google 的 authorize/token endpoint 作为常量自定义,避免 `/google` 子包的重型传递依赖)。
- **会话:** 服务端 `sessions` 表 + httpOnly cookie。cookie 里是原始随机 token,DB 只存其 **sha256**。
- **未配置优雅降级:** 未设置 OAuth env 时应用照常运行,登录端点返回 503,前端提示「登录暂未开放」。
- **限流:** 提交从「按 IP」改为「按登录用户」(30 次/天)。

## 架构与组件

新增 / 改动:

| 单元 | 职责 |
|---|---|
| `internal/auth`(新包) | OAuth provider 配置 + 流程助手:构建授权 URL(带 state)、用 code 换 token、拉 userinfo → 归一为 `Identity{Provider, ProviderUserID, Login, Email, AvatarURL}`。provider 的 authorize/token/userinfo base URL 可注入 → httptest 可测(沿用 `github.Client` 的可测风格)。 |
| `internal/store`(改) | 迁移 `0005_auth.sql`;`UpsertUser`、`CreateSession`、`UserBySessionToken`、`DeleteSession`、`DeleteExpiredSessions`;`InsertSubmission` 增加 `userID` 参数。 |
| `internal/api`(改) | 4 个 auth 端点 + `currentUser(r)` 助手;改 `handleSubmit` 要求登录。 |
| `internal/config`(改) | 新增 OAuth/会话相关 env(全部可选)。 |
| `web/`(改) | `AuthContext` + auth client;Layout 登录态;Submit 页登录门槛。 |

依赖方向不变:`auth → 仅 stdlib + x/oauth2`;`store` 不依赖 auth;`api → {store, auth}`;`config` 独立。

## 数据模型(迁移 `0005_auth.sql`)

```sql
CREATE TABLE users (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  provider         TEXT NOT NULL,            -- 'github' | 'google'
  provider_user_id TEXT NOT NULL,
  login            TEXT NOT NULL,            -- 展示用 handle
  email            TEXT,                     -- 可空
  avatar_url       TEXT,
  created_at       TEXT NOT NULL,
  UNIQUE(provider, provider_user_id)
);

CREATE TABLE sessions (
  id          TEXT PRIMARY KEY,              -- cookie 原始 token 的 sha256(hex)
  user_id     INTEGER NOT NULL REFERENCES users(id),
  created_at  TEXT NOT NULL,
  expires_at  TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

ALTER TABLE submissions ADD COLUMN user_id INTEGER REFERENCES users(id);  -- 旧的纯 IP 行 user_id 为 NULL
```

会话有效期默认 30 天。

## 端点与数据流

| 方法 路径 | 作用 |
|---|---|
| `GET /api/v1/auth/login?provider=github\|google` | 校验 provider 已配置(否则 503);生成随机 `state` 写入短时(10 分钟)httpOnly cookie;302 跳 provider 授权页,redirect_uri = `OAUTH_BASE_URL/api/v1/auth/callback?provider=<p>`。 |
| `GET /api/v1/auth/callback?provider=&code=&state=` | 校验 `state` 与 cookie 一致(否则 400);x/oauth2 `Exchange` 换 token;调 provider userinfo URL 拉身份;`UpsertUser` → `CreateSession` → 种 `session` cookie(httpOnly、SameSite=Lax、https 下 Secure、30 天);清 state cookie;302 回 `/submit`。失败 → 302 回 `/submit?auth_error=1` 并记日志。 |
| `GET /api/v1/auth/me` | 读 session cookie → 返回 `{user:{login,avatar_url,provider}}` 或 `{user:null}`(始终 200)。 |
| `POST /api/v1/auth/logout` | 删 session 行 + 过期清 cookie;204。 |
| `POST /api/v1/submissions`(改) | 读 session;**无 → 401 `{"error":"login required"}`**;有 → 按 user 限流(30/天,超 → 429)→ 校验 full_name → `InsertSubmission(fullName, ip, userID)` → 202 `{id,status:"pending"}`。 |

**登录流程:** 点「用 GitHub 登录」→ 浏览器跳 `/auth/login` → 302 到 provider → 用户授权 → provider 302 回 `/auth/callback` → 服务端换 token、建会话、种 cookie → 302 回 `/submit` → SPA 拉 `/auth/me` 显示已登录。

**提交流程(门槛后):** SPA `POST /submissions`(同源自动带 cookie)→ 服务端 `currentUser` 解析 → 未登录 401(前端回退到登录提示)/ 已登录则记 `user_id` 入 pending。

GitHub:authorize `https://github.com/login/oauth/authorize`、token `https://github.com/login/oauth/access_token`、userinfo `https://api.github.com/user`、scope `read:user`。
Google:authorize `https://accounts.google.com/o/oauth2/v2/auth`、token `https://oauth2.googleapis.com/token`、userinfo `https://openidconnect.googleapis.com/v1/userinfo`、scope `openid email profile`。

## 配置(新增 env,均可选)

| env | 用途 |
|---|---|
| `GITHUB_OAUTH_CLIENT_ID` / `GITHUB_OAUTH_CLIENT_SECRET` | GitHub OAuth App |
| `GOOGLE_OAUTH_CLIENT_ID` / `GOOGLE_OAUTH_CLIENT_SECRET` | Google OAuth client |
| `OAUTH_BASE_URL` | 拼 redirect_uri 的公开基址(本地 `http://localhost:8080`);其 scheme 为 `https` 时 cookie 加 `Secure` |

某 provider 的 id/secret 缺失 → 该 provider 视为未配置,`/auth/login?provider=该项` 返回 503;`/auth/me` 正常返回匿名;Submit 页对未配置的 provider 隐藏其按钮。两者都没配 → 登录整体不可用,应用其余功能照常。

## 前端

- `web/src/auth/`:`AuthContext`(挂载时拉一次 `/auth/me`,共享 `user`/`loading`);`client.ts`(`getMe()`、`login(provider)` = 跳 `/api/v1/auth/login?provider=`、`logout()`)。
- `Layout.tsx`:已登录显示头像 + login + 「登出」;未登录显示「登录」入口。
- `Submit.tsx`:未登录显示「用 GitHub 登录 / 用 Google 登录」按钮(只显示已配置的 provider);已登录显示原表单。提交收到 401 时回退到登录提示。
- auth client `getMe`/`logout` 用 `credentials: 'same-origin'`(同源默认带 cookie)。

## 错误处理

- OAuth 未配置:`/auth/login` 503;`/auth/me` 返回匿名;Submit 页提示「登录暂未开放」。应用其余功能不受影响。
- `state` 不匹配 / 过期 → 400。
- token 交换 / userinfo 失败 → 记日志,302 回 `/submit?auth_error=1`。
- 会话过期 → `/auth/me` 返回匿名;`/submissions` 返回 401。

## 安全

- cookie 全 httpOnly;`SameSite=Lax`(允许从 provider 顶级跳转带回,同时阻止跨站 POST 带 cookie → 作为 CSRF 主防线);https 下加 `Secure`。
- OAuth `state`:128-bit 随机值原样存入短时 httpOnly cookie,callback 时与返回的 `state` 参数比对一致 → 防授权码注入 / CSRF。无需签名密钥(httpOnly 已防 JS 读取/伪造)。
- session token 256-bit 随机;DB 只存 sha256(hex);logout 走 POST。
- 会话过期清理:callback 时顺带 `DeleteExpiredSessions`(无需新增 cron)。
- secrets 仅经 env,绝不打日志。

## 测试

- `internal/auth`:github/google 身份解析、state 不匹配、token 交换失败(httptest 假 provider 的 authorize/token/userinfo)。
- `internal/store`:users/sessions CRUD、会话过期(`UserBySessionToken` 对过期会话返回未找到)、upsert 幂等、带 `user_id` 的提交、`DeleteExpiredSessions`。
- `internal/api`:`/auth/login` 302 + 种 state cookie + Location 含 provider 授权 URL;`/auth/callback` happy path(假 provider)种 session cookie + 302、坏 state → 400;`/auth/me` 有/无 cookie;`/submissions` 无会话 → 401、有会话 → 202 且记 `user_id`、超限 → 429;未配置模式 → `/auth/login` 503。
- 前端:`AuthContext` 渲染;Submit 登录门槛(未登录显示按钮、已登录显示表单);导航登录态。Vitest + RTL,mock auth client。

## 依赖与前置(实施时)

- **一次性联网**:`go get golang.org/x/oauth2 && go mod tidy`(沙箱挡网络,走 `!` 由用户执行,或在允许时禁用沙箱执行)。只 import 核心 `oauth2`,endpoint 用常量,控制传递依赖。
- 需在 GitHub、Google 各注册 OAuth App,回调填 `OAUTH_BASE_URL/api/v1/auth/callback?provider=…`,client_id/secret 填进 env。
- 迁移 `0005` 向后兼容:既有 `submissions` 行 `user_id` 为 NULL。

## 非目标

SSR/SEO;个人主页 / 「我的提交」;邮箱密码登录;刷新 token / 长期 provider token 存储(只在 callback 用一次性 access token 拉身份,不持久化 provider token)。
