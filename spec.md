# Trends — GitHub 趋势仓库追踪平台 · 设计规格（spec）

> 一个 trendshift.io 风格的产品：**在仓库上升期就抓住它**，而不是等它见顶。
> 通过自建的 GitHub 数据采集与动量评分，提供日/周/月/年趋势榜、仓库洞察、可嵌入徽章等能力。

- **状态**：草案 v1（待评审）
- **创建日期**：2026-06-03
- **技术栈**：Go 后端（API + 采集 worker）/ React（Next.js SSR）前端 / SQLite（WAL + Litestream）
- **数据来源**：GitHub REST/GraphQL API + 自建评分（不依赖 GitHub Trending 黑箱）
- **定位**：面向公众的真实产品（SEO、性能、增长、可商业化）

---

## 1. 目标与非目标

### 1.1 产品目标
- 比 GitHub Trending 更早、更细粒度地发现正在起飞的开源项目。
- 提供可解释、可调参的「动量」排名，而非简单按总 star 排序。
- 多时间维度（日/周/月/年）、按语言/话题筛选、单仓库历史洞察。
- 通过可嵌入的趋势徽章形成自然增长回路（README 反向曝光）。
- SEO 友好，每个仓库/话题/语言页都是可被搜索引擎索引的落地页。

### 1.2 成功标准（MVP）
- 每日稳定产出一份可信的趋势榜，覆盖 5 万+ 仓库的追踪宇宙。
- 首页与各期榜单页在服务端渲染、可被 Google 索引。
- 单仓库详情页能展示 star 历史曲线与历史排名。
- 采集任务在 GitHub API 限流内每日全量更新，失败可重试、可续跑。

### 1.3 非目标（本规格不覆盖 / 后置）
- 用户账号体系与个性化订阅（Phase 3）。
- 社交平台「实时提及」（依赖 X/其他平台 API，Phase 3）。
- 赞助位 / 付费墙等商业化具体实现（Phase 3，仅预留埋点）。
- 移动端原生 App。

---

## 2. 范围与分阶段路线图

| 阶段 | 范围 | 交付物 |
|---|---|---|
| **MVP** | 仓库发现 + 每日快照采集 → 动量评分 → 日/周/月榜（首页 + 各期页）→ 语言筛选 → 仓库详情页（star 曲线 + 历史排名）→ 全文搜索 → SSR/SEO/sitemap | 可上线的趋势站 |
| **Phase 1** | 可嵌入 Badge（SVG）+ 仓库提交收录 + 话题/分类页 | 增长引擎 + 内容广度 |
| **Phase 2** | 开发者排名 + Yearly 视图 + Insights/Stats 聚合页 + GitHub Trending 历史归档 | 深度与差异化 |
| **Phase 3** | Live Mentions（社交提及）+ 账号/订阅通知 + 赞助位/商业化 | 商业化与留存 |

本文档对 **MVP 做实现级定义**，对 Phase 1–3 给出接口与数据模型上的预留，避免后续返工。

---

## 3. 系统架构

```
                         ┌────────────────────────────────────────┐
                         │            单一 Go 二进制                 │
   GitHub REST/GraphQL   │  ┌───────────────┐   ┌───────────────┐  │
   ───────────────────►  │  │ 采集 Scheduler │   │   API Server   │  │  ◄──── Next.js (SSR)
       (轮询/搜索)        │  │   (cron jobs)  │   │  (REST/JSON)   │  │        前端
                         │  └───────┬───────┘   └───────┬───────┘  │
                         │          │   共享 *.db 文件    │          │
                         │          └────────┬───────────┘          │
                         │              ┌────▼────┐                  │
                         │              │ SQLite  │ (WAL)            │
                         │              └────┬────┘                  │
                         └───────────────────┼──────────────────────┘
                                             │ WAL streaming
                                        ┌────▼─────┐
                                        │Litestream│──► 对象存储 (S3/R2) 备份/PITR
                                        └──────────┘
```

### 3.1 组件
1. **API Server（Go）**：对外 REST/JSON。只读 SQLite。提供榜单、仓库详情、搜索、语言、（Phase 1+）徽章/话题/提交、（Phase 2+）开发者榜。
2. **采集 Scheduler（Go，同进程内 goroutine）**：cron 触发三类作业（发现 / 快照 / 评分物化）。SQLite 的唯一写者。
3. **SQLite（WAL）**：单一 `.db` 文件，API 与 Scheduler 同机共享。
4. **Next.js 前端（SSR）**：服务端渲染调用 Go API，产出可索引页面与 OG/结构化数据。
5. **Litestream**：将 WAL 持续流式备份到对象存储，提供 PITR/异地容灾。

### 3.2 为什么是「单 Go 二进制 + 内嵌 SQLite」
- 部署 = 一个二进制 + 一个数据文件，运维成本最低。
- 负载画像为「极度读多写少 + 每日批量写」，WAL 下「多读单写」并发足够。
- 唯一约束：API 与 Scheduler 必须访问同一文件 → **单机/单卷**。在需要多写节点前都不是问题（见 §12 迁移触发条件）。

### 3.3 建议目录结构
```
trends/
├── cmd/
│   └── trends/main.go            # 启动 API + Scheduler
├── internal/
│   ├── api/                      # HTTP handlers、路由、DTO
│   ├── ingest/                   # GitHub 客户端、发现/快照作业
│   ├── scoring/                  # 动量评分、排名物化
│   ├── store/                    # SQLite 访问层（database/sql + 可移植 SQL）
│   ├── badge/                    # SVG 徽章生成（Phase 1）
│   └── config/                   # 环境配置
├── migrations/                   # SQL 迁移脚本（编号顺序执行）
├── web/                          # Next.js 前端
│   ├── app/ 或 pages/
│   └── ...
├── spec.md
└── README.md
```

---

## 4. 数据采集与「仓库宇宙」策略

核心难点：要算动量，必须每天给一大批仓库拍快照（star/fork/issue），再算增量。GitHub 认证额度有限，因此**追踪哪些仓库、如何发现新仓库**是架构关键。

### 4.1 采用方案：固定宇宙 + 阈值晋升
维护一个被持续追踪的「仓库宇宙」（目标 5 万–10 万仓库），来源三路：
1. **种子发现**：用 GitHub Search API 按 star 区间 × 语言切片查询，绕过单次 1000 条上限。例如 `stars:100..150 pushed:>{近30天}`、`stars:150..200 ...` 逐段扫。
2. **用户提交**：`POST /submissions`（Phase 1）人工补充。
3. **阈值晋升**：任何已在宇宙中的仓库，若日增量超过阈值，自动标记为「活跃」长期保留；长期沉睡（如 90 天无显著增量且 star 低于门槛）则降级/淘汰，控制宇宙规模与成本。

> 为什么不直接爬 `github.com/trending`：那只是 GitHub 已认定的热门，且算法黑箱、改版易碎，等于退化成 GitHub Trending 的镜像，失去「更早发现」的护城河。

### 4.2 GitHub API 用量与限流
- **认证额度**：GraphQL 5000 点/小时；REST 5000 请求/小时；Search 30 请求/分钟、单查询最多 1000 条结果。
- **快照用 GraphQL 别名批量**：一个请求里 alias 多个 `repository(owner,name)` 节点，单 token 即可日更数万仓库。
- **认证方式**：GitHub App 的 server-to-server token 或 PAT，存于环境变量；支持配置多 token 轮换以提高吞吐。
- **限流处理**：尊重 `X-RateLimit-Remaining` / `Retry-After`，遇 403 二级限流指数退避；作业按 cursor 续跑，幂等可重入。

### 4.3 定时作业（cron）
| 作业 | 频率（默认） | 职责 |
|---|---|---|
| **Discovery** | 每日 1 次 | GitHub Search 扫描候选仓库、处理提交、更新宇宙成员与淘汰 |
| **Snapshot** | 每日 1 次（UTC 00:00） | 对宇宙内所有仓库批量拉取 stars/forks/open_issues/watchers，写入 `repository_snapshots` |
| **Scoring** | 每日 1 次（Snapshot 之后） | 计算增量/EWMA/加速度/分数，物化 `trending_rankings`（日/周/月/年） |
| **Fast-lane**（可选，Phase 1+） | 每小时 | 仅对高动量仓库补采，支撑首页「live」体感 |

所有作业：结构化日志、记录用量指标（采集仓库数 / API 点数 / 错误数）、可断点续跑。

---

## 5. 动量评分算法

设计原则：**透明、可解释、可配置、抗刷分、偏向「上升」而非「已见顶」**。

### 5.1 每日基础量（对仓库 r、日期 d）
- `star_delta_d = stars_d − stars_{d-1}`（fork、issue 同理）
- `star_ewma_d`：近 7 个日增量的指数加权移动平均，平滑因子 α（默认 0.5），抑制单日噪声
- `acceleration_d = Σ(近7日 star_delta) − Σ(前7日 star_delta)`：捕捉「正在加速」

### 5.2 归一化（避免大仓库恒霸榜）
对绝对增量做对数压缩，并引入**相对增长**给中小仓库机会：
```
rel_growth_d = star_delta_d / log10(stars_d + 10)
```

### 5.3 综合分数
```
score_d =  w1 · norm(star_ewma_d)
         + w2 · max(0, norm(acceleration_d))
         + w3 · norm(fork_delta_ewma_d)
         + w4 · norm(rel_growth_d)
         − decay(repo_age, days_since_peak)
```
- `norm(x)`：对当日参与计分的仓库群做 min-max 缩放到 `[0,1]`（按当日 cohort 归一，使分数跨日可比、不被极值拉爆）。
- 权重 `w1..w4` 与 `decay` 系数全部进配置文件，便于离线调参。
- 默认侧重 `acceleration` 与 `rel_growth`，以体现「caught as they rise」。
- **抗刷分**：对单日 star 异常尖峰做温莎化（winsorize）截断；新建（< N 天）仓库设最低 star 门槛后再计分。

### 5.4 各期排名物化
- **Daily**：按 `score_d` 对日期 d 排序。
- **Weekly / Monthly / Yearly**：以对应窗口（7/30/365 天）内累计增量与窗口分数排序。
- 每期 Top-N（默认 N=200）落库到 `trending_rankings`，前端只读物化结果，**不在请求时实时计算**。

---

## 6. 数据模型（SQLite）

约定：使用标准 SQL 与 `database/sql`，避免任何方言特性，为日后迁移 Postgres 留后路；不使用数组列（用关联表代替）。所有时间用 UTC、ISO8601 文本或 Unix 秒。

```sql
-- 6.1 仓库主表
CREATE TABLE repositories (
    id              INTEGER PRIMARY KEY,         -- 内部自增 ID（对外、徽章、URL 使用）
    github_id       INTEGER NOT NULL UNIQUE,     -- GitHub 仓库数字 ID（稳定，重命名不变）
    full_name       TEXT    NOT NULL UNIQUE,     -- owner/name
    owner           TEXT    NOT NULL,
    name            TEXT    NOT NULL,
    description     TEXT,
    language        TEXT,                        -- 主语言
    homepage        TEXT,
    html_url        TEXT    NOT NULL,
    owner_avatar    TEXT,
    stars           INTEGER NOT NULL DEFAULT 0,  -- 最新一次快照值（冗余，便于排序/展示）
    forks           INTEGER NOT NULL DEFAULT 0,
    open_issues     INTEGER NOT NULL DEFAULT 0,
    is_archived     INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 1,  -- 宇宙成员状态（晋升/淘汰）
    repo_created_at TEXT,                        -- GitHub 上的创建时间
    first_seen_at   TEXT    NOT NULL,
    last_synced_at  TEXT,
    best_rank       INTEGER,                     -- 历史最佳日榜名次（徽章/洞察用）
    best_rank_at    TEXT
);
CREATE INDEX idx_repos_language ON repositories(language);
CREATE INDEX idx_repos_active   ON repositories(is_active);
CREATE INDEX idx_repos_stars    ON repositories(stars DESC);

-- 6.2 每日快照（时间序列）
CREATE TABLE repository_snapshots (
    repository_id   INTEGER NOT NULL REFERENCES repositories(id),
    snapshot_date   TEXT    NOT NULL,            -- YYYY-MM-DD (UTC)
    stars           INTEGER NOT NULL,
    forks           INTEGER NOT NULL,
    open_issues     INTEGER NOT NULL,
    watchers        INTEGER NOT NULL DEFAULT 0,
    star_delta      INTEGER NOT NULL DEFAULT 0,  -- 相对前一快照（采集时算好，省查询）
    PRIMARY KEY (repository_id, snapshot_date)
);
CREATE INDEX idx_snapshots_date ON repository_snapshots(snapshot_date);

-- 6.3 趋势榜（物化结果）
CREATE TABLE trending_rankings (
    period          TEXT    NOT NULL,            -- 'daily'|'weekly'|'monthly'|'yearly'
    period_date     TEXT    NOT NULL,            -- 该期的代表日期（如周一/月初/日）
    repository_id   INTEGER NOT NULL REFERENCES repositories(id),
    rank            INTEGER NOT NULL,
    score           REAL    NOT NULL,
    star_delta      INTEGER NOT NULL,            -- 该窗口内 star 增量（展示用）
    language        TEXT,                        -- 冗余，支持按语言过滤榜单
    PRIMARY KEY (period, period_date, repository_id)
);
CREATE INDEX idx_rankings_lookup ON trending_rankings(period, period_date, rank);
CREATE INDEX idx_rankings_lang   ON trending_rankings(period, period_date, language, rank);

-- 6.4 提交收录（Phase 1）
CREATE TABLE submissions (
    id              INTEGER PRIMARY KEY,
    full_name       TEXT    NOT NULL,            -- owner/name
    status          TEXT    NOT NULL DEFAULT 'pending', -- pending|accepted|rejected
    submitted_ip    TEXT,
    created_at      TEXT    NOT NULL
);

-- 6.5 话题（Phase 1）
CREATE TABLE topics (
    id          INTEGER PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,            -- 'ai-agent'
    name        TEXT NOT NULL                    -- 'AI agent'
);
CREATE TABLE repository_topics (
    repository_id INTEGER NOT NULL REFERENCES repositories(id),
    topic_id      INTEGER NOT NULL REFERENCES topics(id),
    PRIMARY KEY (repository_id, topic_id)
);

-- 6.6 开发者排名（Phase 2）
CREATE TABLE developers (
    id              INTEGER PRIMARY KEY,
    github_login    TEXT NOT NULL UNIQUE,
    avatar          TEXT,
    appearances     INTEGER NOT NULL DEFAULT 0   -- 累计上榜次数（物化）
);
```

> **快照增长控制**：`repository_snapshots` 随时间线性增长（如 10 万仓库 × 365 天 ≈ 3650 万行/年）。SQLite 加索引可承受；超过约定阈值后对 90 天前的日快照做 rollup（降采样为周/月），原始日级仅保留近 N 天。该 rollup 作业列入 Phase 2。

---

## 7. API 设计（REST/JSON）

基址 `/api/v1`。所有列表接口支持 `page`（默认 1）与 `per_page`（默认 25，上限 100）。响应带 HTTP 缓存头（`Cache-Control`、`ETag`）。

### MVP
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/trending` | 趋势榜。Query：`period=daily\|weekly\|monthly`（MVP，`yearly` 见 Phase 2）、`language`、`date`、分页。读 `trending_rankings`。 |
| GET | `/api/v1/repositories/{id}` | 仓库详情（含最新指标、最佳名次、话题）。 |
| GET | `/api/v1/repositories/{id}/snapshots` | star/fork 历史序列，画曲线用。Query：`from`、`to`。 |
| GET | `/api/v1/repositories/{id}/rankings` | 该仓库的历史上榜记录。 |
| GET | `/api/v1/search` | 全文搜索（按 `full_name`/`description`）。Query：`q`、`language`、分页。 |
| GET | `/api/v1/languages` | 语言列表及各自在榜数量（用于筛选 Tab）。 |
| GET | `/healthz` | 健康检查（DB 可达、最近一次成功采集时间）。 |
| GET | `/sitemap.xml`, `/robots.txt` | SEO。 |

### Phase 1+
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/repositories/{id}/badge.svg` | 动态生成的趋势徽章（见 §9）。 |
| POST | `/api/v1/submissions` | 提交一个仓库收录（带防滥用限流）。 |
| GET | `/api/v1/topics`, `/api/v1/topics/{slug}` | 话题列表 / 话题下趋势仓库。 |
| GET | `/api/v1/developers` | 开发者排名（Phase 2）。Query：`period`。 |

**响应示例**（`GET /api/v1/trending?period=daily`）：
```json
{
  "period": "daily",
  "date": "2026-06-03",
  "page": 1, "per_page": 25, "total": 200,
  "items": [
    {
      "rank": 1,
      "score": 87.4,
      "star_delta": 1320,
      "repository": {
        "id": 17415,
        "full_name": "obra/superpowers",
        "description": "…",
        "language": "TypeScript",
        "stars": 24050,
        "html_url": "https://github.com/obra/superpowers",
        "owner_avatar": "https://…",
        "topics": ["ai-agent", "developer-tools"]
      }
    }
  ]
}
```

---

## 8. 前端（Next.js / SSR）

### 8.1 页面与路由
| 路由 | 内容 | 渲染 |
|---|---|---|
| `/` | 首页：当日趋势榜 + 时间维度切换 + 语言 Tab | SSR/ISR |
| `/trending/[period]` | `daily`/`weekly`/`monthly` 各期榜单 | SSR/ISR |
| `/languages/[lang]` | 按语言的趋势榜落地页 | SSR/ISR |
| `/repositories/[id]` | 仓库详情：star 曲线 + 历史排名 + 元信息 | SSR |
| `/search` | 搜索结果 | SSR |
| `/topics`, `/topics/[slug]` | 话题页（Phase 1） | SSR/ISR |
| `/submit` | 提交收录（Phase 1） | CSR + API |
| `/trending/developers` | 开发者榜（Phase 2） | SSR/ISR |
| `/about` | 方法论说明（评分如何计算，建立信任） | 静态 |

### 8.2 SEO 策略（公众产品关键）
- 全部内容页 **服务端渲染**，确保可被索引；列表页用 ISR（增量静态再生）降低源站压力。
- 每页注入 `<title>`/`meta description`/OpenGraph/Twitter Card；仓库页加 schema.org 结构化数据。
- `/sitemap.xml` 动态生成，覆盖所有仓库/语言/话题页；`robots.txt` 放行。
- 语义化 URL（`/languages/python`、`/topics/ai-agent`）。
- 隐私友好：无追踪像素、无弹窗（与 trendshift 一致的产品调性）。

---

## 9. 可嵌入徽章 Badge（Phase 1）

增长引擎：作者把徽章放进 README，带来反向曝光。

- 端点 `GET /api/v1/repositories/{id}/badge.svg`，返回 SVG（参考 shields.io 风格）。
- 内容：项目历史最佳名次或当前名次，如「Trends Rank #3」。
- 缓存：`Cache-Control` + `ETag`，并在服务端缓存渲染结果；徽章数据来自 `repositories.best_rank` / 当日 `trending_rankings`。
- 提供一键复制的 Markdown 嵌入代码（仓库详情页）。
- 防滥用：仅对已收录仓库出图，未知 `id` 返回占位徽章。

---

## 10. 非功能需求

- **性能**：榜单全部预计算物化；API 加 HTTP 缓存头，前置 CDN；SQLite 读取走 WAL，常用查询有覆盖索引。目标 P95 API < 50ms（缓存命中外）。
- **可靠性**：采集作业幂等、可断点续跑；GitHub 调用指数退避重试；作业失败告警但不影响只读 API（站点继续展示上一份榜单）。
- **数据备份**：Litestream 持续流式备份 WAL 到对象存储，支持 PITR。
- **安全**：密钥仅经环境变量注入，支持 GitHub token 轮换；提交接口与徽章接口限流防滥用；所有输入校验；不存储任何用户 PII（MVP 无账号）。
- **可观测性**：结构化日志；作业指标（采集仓库数、消耗 API 点数、错误率、各作业耗时）；`/healthz` 暴露最近成功采集时间。
- **配置**：评分权重、宇宙规模阈值、cron 表达式、token 列表均可配置，无需改代码。

---

## 11. 部署与运维

- **后端**：单 Go 二进制（API + Scheduler），跑在一台 VPS / 容器；持久卷挂载 `.db`；旁路运行 Litestream。
- **前端**：Next.js 以 standalone 模式部署（同机 Node 运行时或托管平台），通过内网/公网调用 Go API。
- **回滚**：二进制可快速回退；DB 借 Litestream 做时点恢复。
- **环境变量（示例）**：`GITHUB_TOKENS`、`DB_PATH`、`LITESTREAM_*`、`SCORING_WEIGHTS`、`CRON_*`、`API_LISTEN_ADDR`。

---

## 12. 迁移到 Postgres 的触发条件

保持 SQL 可移植，满足以下任一即评估迁移：
- 需要**多写节点**或多区域写入。
- 写并发显著上升（如引入实时高频写入）。
- 单库体积超过约定阈值（如 > 数十 GB）且 rollup 后仍吃紧。
- 需要 SQLite 不便支撑的能力（强并发分析、扩展生态）。

过渡可选项：分布式 SQLite（LiteFS / Turso）作为中间形态，避免一步到位改 Postgres。

---

## 13. 风险与开放问题

| 项 | 风险 | 缓解 |
|---|---|---|
| GitHub 限流 | 宇宙扩大后额度紧张、二级限流 | GraphQL 别名批量、多 token 轮换、退避重试、续跑 |
| 评分调参 | 冷启动榜单质量不稳、可被刷分 | winsorize 截断、最低门槛、权重离线调参、`/about` 公开方法论 |
| 宇宙覆盖 | 漏掉冷门但正起飞的项目 | 阈值晋升 + 用户提交 + 定期扩大 Search 切片 |
| 话题分类来源（Phase 1） | GitHub topics 噪声大 | 先用 GitHub topics，再叠加人工策展白名单 |
| Live Mentions（Phase 3） | X/社交 API 成本与政策 | 后置评估，必要时换数据源或降级 |
| 快照体量 | 时间序列线性膨胀 | Phase 2 引入 rollup 降采样 |

**待确认的开放问题**（不阻塞 MVP）：
1. 宇宙目标规模先定 5 万还是 10 万？（影响采集时长与成本）
2. 评分默认权重的初值与冷启动观察期长度？
3. 前端 UI 视觉风格是否需要专门设计稿，还是先用简洁默认主题？

---

## 14. 里程碑

- **M0 — 数据地基**：GitHub 客户端 + Discovery/Snapshot 作业 + SQLite schema/迁移 + Litestream。产出：宇宙建立、每日快照可跑。
- **M1 — 评分与榜单**：Scoring 作业 + `trending_rankings` 物化 + `/api/v1/trending` & `/repositories/*` API。产出：可查的日/周/月榜。
- **M2 — 前端与 SEO**：Next.js 首页/各期页/详情页/搜索 + SSR + sitemap/OG。产出：**可上线的 MVP**。
- **M3 — 增长（Phase 1）**：Badge + 提交收录 + 话题页。

---

*本规格为 MVP 实现级定义 + 后续阶段接口预留。评审通过后进入实现计划（writing-plans）。*
