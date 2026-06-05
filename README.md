# Trends — GitHub 趋势仓库追踪平台

一个 trendshift.io 风格的产品:在仓库上升期就抓住它。详见 [spec.md](./spec.md);分阶段实现计划在 [docs/superpowers/plans/](./docs/superpowers/plans/);给 AI agent 的工作约定见 [AGENTS.md](./AGENTS.md)。

当前实现:**M0(数据地基)+ M1(评分与只读 API)+ M2(React SPA 前端)**。

## 架构

单一 Go 二进制 = cron 采集/评分 worker + 只读 REST API + 内嵌的 React SPA,全部跑在内嵌 SQLite(WAL)之上。

- **Discovery**:GitHub REST Search 按 star 区间切片发现仓库,upsert 进 SQLite。
- **Snapshot**:GitHub GraphQL `nodes(ids:)` 批量拉 star/fork/issue,算每日 `star_delta`,写日快照。
- **Scoring**:基于快照算动量分(ewma + 加速度 + 窗口增量 + 相对增长,cohort 归一,可配权重),把日/周/月 Top-N 物化进 `trending_rankings`(每日快照后自动触发)。
- **API**(只读,`/api/v1`):`trending`、`languages`、`repositories/{id}`(+`/snapshots`、`/rankings`)、`search`,外加 `/healthz`。
- **前端**:Vite + React + TS + Tailwind 的客户端渲染 SPA(`web/`),构建产物 `go:embed` 进二进制由 API Server 一并托管(非 API 路径回退 `index.html`)。**无 SSR/SEO**。

## 配置(环境变量)

| 变量 | 默认值 | 说明 |
|---|---|---|
| `DB_PATH` | `trends.db` | SQLite 文件路径 |
| `GITHUB_TOKENS` | (空) | 逗号分隔的 GitHub token,多 token 轮询;留空则不鉴权、额度很低 |
| `GITHUB_API_BASE_URL` | `https://api.github.com` | REST API 基址 |
| `GITHUB_GRAPHQL_URL` | `https://api.github.com/graphql` | GraphQL 端点 |
| `API_LISTEN_ADDR` | `:8080` | HTTP 监听地址 |
| `DISCOVERY_CRON` | `0 1 * * *` | 发现作业 cron |
| `SNAPSHOT_CRON` | `0 0 * * *` | 快照作业 cron(成功后链式触发评分) |
| `DISCOVERY_QUERIES` | 6 段 star 区间切片 | 发现用的 GitHub Search 查询集,**换行或逗号分隔**。默认按 star 切片(绕开单查询 1000 条上限),门槛 ≥50、宇宙约 5k。可整体覆盖成按语言/活跃度切片,如 `language:go stars:>50` 或 `pushed:>2026-01-01 stars:100..500` |
| `DISCOVERY_MAX_PAGES` | `10` | 每条查询最多翻几页(每页 100 条,sort=stars desc);越大挖得越深、API 调用越多 |

## 构建与测试

依赖已缓存,Go 构建/测试全程离线;仅 `npm install`(前端依赖,一次性)需网络。

    cd web && npm install        # 一次性,需网络
    make test                    # 前端(Vitest)+ 后端(go test)全部测试
    make build                   # 构建前端 → 嵌入 → 输出 bin/trends(单二进制)

仅后端(不碰前端):`GOPROXY=off go test ./cmd/... ./internal/...`(用此范围而非 `./...`,避免扫描 `web/node_modules` 里的 .go 文件)。

## 本地运行

常驻(cron 调度 + API + 前端,默认 :8080):

    GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db ./bin/trends
    # 打开 http://localhost:8080

手动跑一次作业(便于验证):

    RUN_ONCE=discovery GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends
    RUN_ONCE=snapshot  ... go run ./cmd/trends
    RUN_ONCE=score     ... go run ./cmd/trends

### 前端开发

    cd web && npm run dev        # Vite dev server;/api 与 /healthz 代理到 :8080(需后端在跑)

## 备份(生产)

用 [Litestream](https://litestream.io/) 把 SQLite WAL 持续备份到对象存储:

    litestream replicate -config litestream.yml

`litestream.yml` 中的 `${...}` 占位在部署环境用真实值或环境变量注入。
