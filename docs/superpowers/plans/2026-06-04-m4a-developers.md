# M4a 开发者排名 实现计划

> 控制器直实现,逐任务 TDD + 验证 + 独立提交。Phase 2 的第一块。

**Goal:** 按开发者(owner)登上日榜的累计次数排名:`GET /api/v1/developers?period=`(分页),前端 `/trending/developers` 页。

**Architecture:** 不新建 `developers` 表(spec §6.6 的物化表延后);在 store 用 `trending_rankings JOIN repositories GROUP BY owner` **实时聚合**(数据量小、有索引,够用;成为热点再物化)。api 加只读端点,前端加页面。

**Tech Stack:** Go 标准库 · React/TS · Vitest。无新增依赖。

> **依赖:** M0–M3c 在 `main`。已有 `trending_rankings`、`repositories(owner, owner_avatar)`、api `Server`/分页约定/`validPeriods`、前端 client/路由/Layout。

---

## Task 1: store — 开发者聚合查询

**Files:** `internal/store/developer.go`、`internal/store/developer_test.go`

类型与方法:
```go
type Developer struct {
	Login       string
	Avatar      string
	Appearances int
}
```
- `ListDevelopers(period string, limit, offset int) ([]Developer, error)`:
  `SELECT r.owner, MAX(r.owner_avatar), COUNT(*) FROM trending_rankings tr JOIN repositories r ON r.id=tr.repository_id WHERE tr.period=? GROUP BY r.owner ORDER BY COUNT(*) DESC, r.owner LIMIT ? OFFSET ?`
- `CountDevelopers(period string) (int, error)`:
  `SELECT COUNT(*) FROM (SELECT 1 FROM trending_rankings tr JOIN repositories r ON r.id=tr.repository_id WHERE tr.period=? GROUP BY r.owner)`

测试:seed owner `alice` 的仓库在 06-09、06-10 各上一次日榜(2 次),`bob` 1 次,`carol` 只在 weekly(daily 不计)。断言 `ListDevelopers("daily")` → `[{alice,_,2},{bob,_,1}]`、`CountDevelopers("daily")==2`、分页。

---

## Task 2: api — GET /api/v1/developers

**Files:** `internal/api/developers.go`、`internal/api/developers_test.go`、改 `server.go`(路由)

handler:`period`(默认 daily,`validPeriods` 校验,非法 400)+ `page`/`per_page`(钳制);调 `CountDevelopers`/`ListDevelopers`;响应 `{period,page,per_page,total,items:[{login,avatar,appearances}]}`(items 非 nil → `[]`)。路由 `GET /api/v1/developers`(与现有不冲突)。

测试:seed → 列表按 appearances 降序、计数、空数组、bad period 400。

---

## Task 3: 前端 /trending/developers + e2e + 收尾

**Files:** `web/src/api/{types,client}.ts`(Developer/DevelopersResponse + getDevelopers)、`web/src/pages/Developers.tsx` + 测试、改 `App.tsx`(路由 `/trending/developers`)、`Layout.tsx`(导航「开发者」)

- client:`getDevelopers(period?, page?)`;types:`Developer{login,avatar,appearances}`、`DevelopersResponse`。
- Developers 页:列表(头像 + login 链接到 GitHub `https://github.com/{login}` + appearances 次数)。
- 路由 `/trending/developers`;Layout 导航加链接。
- 测试:mock client,断言开发者列表渲染 + 次数。
- e2e:`make web && go build`,起服务 curl `/api/v1/developers`(空 DB → `{...,"items":[]}` 200)、`/trending/developers`(SPA 200)。清理 embed。提交收尾。

---

## Self-Review(已核对)
- **Spec 覆盖**:spec §7 `/api/v1/developers`、§8.1 `/trending/developers`。**有意偏离**:spec §6.6 的 `developers` 物化表延后,改实时聚合(更简单;数据量小;`trending_rankings` 有 period+date 索引)。若聚合成为热点,再加物化表 + 重算作业(类比 trending_rankings)。
- **语义**:appearances = 该 owner 的仓库在指定 period 榜上的行数累计(≈ trendshift「featured X times」);avatar 取 MAX(任一)。
- **安全/一致**:period 白名单;绑定参数;per_page 钳制;复用分页/DTO 约定。

## Execution Handoff
Plan complete and saved to `docs/superpowers/plans/2026-06-04-m4a-developers.md`.
