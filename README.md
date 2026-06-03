# Trends — GitHub 趋势仓库追踪平台

详见 [spec.md](./spec.md)。当前实现:**M0 数据地基**(见 [docs/superpowers/plans/2026-06-03-m0-data-foundation.md](./docs/superpowers/plans/2026-06-03-m0-data-foundation.md))。

## 架构(M0)

单一 Go 二进制,内含 cron 调度器,跑两个作业:

- **Discovery**:用 GitHub REST Search 按 star 区间切片发现仓库,upsert 进 SQLite。
- **Snapshot**:用 GitHub GraphQL `nodes(ids:)` 批量拉取 star/fork/issue,计算每日 `star_delta`,写日快照并更新仓库冗余字段。

数据存于内嵌 SQLite(WAL 模式)。

## 配置(环境变量)

| 变量 | 默认值 | 说明 |
|---|---|---|
| `DB_PATH` | `trends.db` | SQLite 文件路径 |
| `GITHUB_TOKENS` | (空) | 逗号分隔的 GitHub token,多 token 轮询;留空则不鉴权、额度很低 |
| `GITHUB_API_BASE_URL` | `https://api.github.com` | REST API 基址 |
| `GITHUB_GRAPHQL_URL` | `https://api.github.com/graphql` | GraphQL 端点 |
| `DISCOVERY_CRON` | `0 1 * * *` | 发现作业 cron |
| `SNAPSHOT_CRON` | `0 0 * * *` | 快照作业 cron |

## 构建与测试

    go build ./...
    go test ./...

## 本地运行

手动跑一次作业(便于验证):

    RUN_ONCE=discovery GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends
    RUN_ONCE=snapshot  GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends

常驻(按 cron 调度):

    GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends

## 备份(生产)

用 [Litestream](https://litestream.io/) 把 SQLite WAL 持续备份到对象存储:

    litestream replicate -config litestream.yml

`litestream.yml` 中的 `${...}` 占位在部署环境用真实值或环境变量注入。
