# M1a 评分与榜单 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 M0 采集的每日快照,计算透明可调的「动量分」,把日/周/月趋势榜物化进 `trending_rankings`,并接入调度(每日快照后自动评分)。

**Architecture:** 新增纯函数 `scoring` 包(无 DB 依赖,易测);`store` 增加榜单写入(`ReplaceRankings`,按 period+date 幂等替换)与评分输入读取(每日增量序列 + 仓库元信息);`ingest` 新增 `RunScoring` 作业把两者串起来,对每个周期算分→排序→取 Top-N→物化;`cmd/trends` 在每日快照成功后链式触发评分。

**Tech Stack:** Go 1.26 · `modernc.org/sqlite` · `github.com/stretchr/testify` · 标准库 `math`/`sort`/`time`

> **依赖:** 本计划基于已合并到 `main` 的 M0(包 `config`/`store`/`github`/`ingest`/`scheduler`/`cmd/trends`)。`store.DB` 已有 `UpsertRepository`、`UpdateRepositoryMetrics`、`InsertSnapshot`、`StarsBefore`、`SQL()`;`repository_snapshots(repository_id, snapshot_date, stars, forks, open_issues, watchers, star_delta)` 与 `repositories(id, github_id, language, stars, is_active, …)` 已存在。
> **离线构建:** 依赖已在模块缓存中。用 `GOPROXY=off go build/test`;不要 `go get`。

## 范围与评分模型决策(M1a,均符合 spec §5/§5.4,刻意从简、可调)

- **周期**:`daily`(窗口 1 天)、`weekly`(7 天)、`monthly`(30 天)。`yearly` 按 spec §7 属 Phase 2,不做。
- **信号(全部由已存的 `star_delta`/`stars` 算出,无需改采集)**,对周期窗口 W、截止日 D:
  - `windowDelta` = 窗口 `[D-W+1, D]` 内 `star_delta` 之和(榜单展示的增量)。
  - `ewma` = 窗口内每日 `star_delta`(按日期升序)的指数加权移动平均,平滑因子 `alpha`。
  - `accel` = `windowDelta` − 前一个等长窗口 `[D-2W+1, D-W]` 的增量和(加速度)。
  - `relGrowth` = `windowDelta / log10(stars + 10)`(给中小仓库机会)。
- **归一化**:各信号在「当日候选集」内做 min-max 到 `[0,1]`(全相等则全 0)。
- **候选过滤**:`stars >= MinStars` 且 `windowDelta > 0`(必须净增长才上榜)。
- **分数**:`score = w_ewma·N(ewma) + w_accel·N(accel) + w_window·N(windowDelta) + w_rel·N(relGrowth)`。权重、`alpha`、`MinStars`、`TopN`、各周期天数全部进 `scoring.Config`(spec「便于离线调参」)。
- **刻意推迟到后续调参**:fork 分量(spec 的 `w3`,需要 fork 日增量,M0 未单独存,留待 M1 调参或加列)、winsorize 截断、衰减项 `decay`、per-period 权重。M1a 先把「可解释、可物化、可调」的骨架立起来。

---

## File Structure

| 文件 | 职责 |
|---|---|
| `internal/store/migrations/0002_trending_rankings.sql` | `trending_rankings` 建表 + 索引 |
| `internal/store/ranking.go` | `Ranking` 类型 + `ReplaceRankings`(按 period+date 事务替换) |
| `internal/store/ranking_test.go` | 榜单写入/替换测试 |
| `internal/store/scoring_input.go` | `DeltaRow`/`DailyDeltasInRange` + `RepoMeta`/`ActiveRepoMeta` |
| `internal/store/scoring_input_test.go` | 评分输入读取测试 |
| `internal/scoring/scoring.go` | `Config`/`Weights`/`DefaultConfig` + 类型 + 纯信号函数(`AddDays`/`ewma`/`relGrowth`/`windowSignals`/`normalize`) |
| `internal/scoring/signals_test.go` | 信号纯函数测试 |
| `internal/scoring/rank.go` | `RankPeriod`(候选过滤 + 归一 + 加权 + 排序 + TopN) |
| `internal/scoring/rank_test.go` | 排名逻辑测试 |
| `internal/ingest/scoring.go` | `RunScoring` 作业(加载→构建输入→逐周期排名→物化) |
| `internal/ingest/scoring_test.go` | 作业集成测试(临时 DB) |
| `cmd/trends/main.go`(改) | 快照成功后链式评分;`RUN_ONCE=score` |

---

## Task 1: trending_rankings 迁移 + 榜单写入

**Files:**
- Create: `internal/store/migrations/0002_trending_rankings.sql`
- Create: `internal/store/ranking.go`
- Test: `internal/store/ranking_test.go`

- [ ] **Step 1: 写迁移 SQL**

`internal/store/migrations/0002_trending_rankings.sql`:
```sql
CREATE TABLE trending_rankings (
    period          TEXT    NOT NULL,           -- 'daily'|'weekly'|'monthly'
    period_date     TEXT    NOT NULL,           -- YYYY-MM-DD (UTC)
    repository_id   INTEGER NOT NULL REFERENCES repositories(id),
    rank            INTEGER NOT NULL,
    score           REAL    NOT NULL,
    star_delta      INTEGER NOT NULL,
    language        TEXT,
    PRIMARY KEY (period, period_date, repository_id)
);
CREATE INDEX idx_rankings_lookup ON trending_rankings(period, period_date, rank);
CREATE INDEX idx_rankings_lang   ON trending_rankings(period, period_date, language, rank);
```

- [ ] **Step 2: 写失败的测试**

`internal/store/ranking_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceRankingsInsertsRows(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	err = db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 120, Language: "Go"},
	})
	require.NoError(t, err)

	var rank, delta int
	var score float64
	err = db.SQL().QueryRow(
		`SELECT rank, score, star_delta FROM trending_rankings WHERE period=? AND period_date=? AND repository_id=?`,
		"daily", "2026-06-10", id).Scan(&rank, &score, &delta)
	require.NoError(t, err)
	require.Equal(t, 1, rank)
	require.Equal(t, 0.9, score)
	require.Equal(t, 120, delta)
}

func TestReplaceRankingsIsIdempotentPerPeriodDate(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.5, StarDelta: 50, Language: "Go"},
	}))
	// 同 period+date 重跑:整组替换,不残留旧行、不报主键冲突
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.8, StarDelta: 99, Language: "Go"},
	}))

	var count, delta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT COUNT(*), MAX(star_delta) FROM trending_rankings WHERE period=? AND period_date=?`,
		"daily", "2026-06-10").Scan(&count, &delta))
	require.Equal(t, 1, count)
	require.Equal(t, 99, delta)
}

func TestReplaceRankingsEmptyClearsAndInsertsNothing(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.5, StarDelta: 50, Language: "Go"},
	}))
	// 用空切片重跑 → 清空该 period+date 的旧行
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", nil))

	var count int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT COUNT(*) FROM trending_rankings WHERE period=? AND period_date=?`,
		"daily", "2026-06-10").Scan(&count))
	require.Equal(t, 0, count)
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run TestReplaceRankings -v`
Expected: FAIL — `undefined: Ranking` / `ReplaceRankings`。

- [ ] **Step 4: 实现榜单写入**

`internal/store/ranking.go`:
```go
package store

// Ranking 是 trending_rankings 的一行(物化的榜单项)。
type Ranking struct {
	RepositoryID int64
	Rank         int
	Score        float64
	StarDelta    int
	Language     string
}

// ReplaceRankings 在单个事务里,用 rows 整体替换 (period, periodDate) 的榜单:
// 先删除该 period+date 的所有旧行,再插入新行。对重跑幂等;rows 为空则仅清空。
func (d *DB) ReplaceRankings(period, periodDate string, rows []Ranking) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM trending_rankings WHERE period=? AND period_date=?`,
		period, periodDate); err != nil {
		tx.Rollback()
		return err
	}
	for _, r := range rows {
		if _, err := tx.Exec(`
INSERT INTO trending_rankings
  (period, period_date, repository_id, rank, score, star_delta, language)
VALUES (?,?,?,?,?,?,?)`,
			period, periodDate, r.RepositoryID, r.Rank, r.Score, r.StarDelta, r.Language); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS（含原有 store 测试）。

- [ ] **Step 6: 提交**

```bash
git add internal/store/ranking.go internal/store/ranking_test.go internal/store/migrations/0002_trending_rankings.sql
git commit -m "$(printf 'feat(store): trending_rankings table and ReplaceRankings\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 2: 评分输入读取(每日增量序列 + 仓库元信息)

**Files:**
- Create: `internal/store/scoring_input.go`
- Test: `internal/store/scoring_input_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/store/scoring_input_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyDeltasInRangeOrdersByRepoThenDate(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-08", Stars: 100, StarDelta: 10}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-09", Stars: 130, StarDelta: 30}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-10", Stars: 150, StarDelta: 20}))

	// 只取 06-09..06-10
	rows, err := db.DailyDeltasInRange("2026-06-09", "2026-06-10")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "2026-06-09", rows[0].Date)
	require.Equal(t, 30, rows[0].StarDelta)
	require.Equal(t, "2026-06-10", rows[1].Date)
	require.Equal(t, 20, rows[1].StarDelta)
	require.Equal(t, id, rows[0].RepositoryID)
}

func TestActiveRepoMetaReturnsLanguageAndStars(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo()) // sampleRepo: Language "Go"
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(111, 1000, 50, 5, 9, "2026-06-10T00:00:00Z"))

	meta, err := db.ActiveRepoMeta()
	require.NoError(t, err)
	m, ok := meta[id]
	require.True(t, ok)
	require.Equal(t, "Go", m.Language)
	require.Equal(t, 1000, m.Stars)
	require.Equal(t, id, m.ID)
}
```
（`sampleRepo()` 定义在 `repository_test.go`,`github_id` 为 111,`Language` 为 `"Go"`;`newTestDB` 在 `store_test.go`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/store/ -run "TestDailyDeltasInRange|TestActiveRepoMeta" -v`
Expected: FAIL — `undefined: DeltaRow`/`DailyDeltasInRange`/`RepoMeta`/`ActiveRepoMeta`。

- [ ] **Step 3: 实现读取**

`internal/store/scoring_input.go`:
```go
package store

// DeltaRow 是某仓库某天的 star 增量(评分输入)。
type DeltaRow struct {
	RepositoryID int64
	Date         string
	StarDelta    int
}

// DailyDeltasInRange 返回 [from, to] 内所有快照的 star 增量,按 (repository_id, snapshot_date) 升序。
func (d *DB) DailyDeltasInRange(from, to string) ([]DeltaRow, error) {
	rows, err := d.db.Query(`
SELECT repository_id, snapshot_date, star_delta
FROM repository_snapshots
WHERE snapshot_date >= ? AND snapshot_date <= ?
ORDER BY repository_id, snapshot_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeltaRow
	for rows.Next() {
		var r DeltaRow
		if err := rows.Scan(&r.RepositoryID, &r.Date, &r.StarDelta); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RepoMeta 是评分需要的仓库元信息。
type RepoMeta struct {
	ID       int64
	Language string
	Stars    int
}

// ActiveRepoMeta 返回所有活跃仓库的 id->元信息(语言 + 当前 star 数)。
func (d *DB) ActiveRepoMeta() (map[int64]RepoMeta, error) {
	rows, err := d.db.Query(`
SELECT id, COALESCE(language,''), stars
FROM repositories WHERE is_active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]RepoMeta)
	for rows.Next() {
		var m RepoMeta
		if err := rows.Scan(&m.ID, &m.Language, &m.Stars); err != nil {
			return nil, err
		}
		out[m.ID] = m
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/store/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/store/scoring_input.go internal/store/scoring_input_test.go
git commit -m "$(printf 'feat(store): scoring input loaders (daily deltas, active repo meta)\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 3: 评分配置 + 纯信号函数

**Files:**
- Create: `internal/scoring/scoring.go`
- Test: `internal/scoring/signals_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/scoring/signals_test.go`:
```go
package scoring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddDays(t *testing.T) {
	out, err := AddDays("2026-06-10", -2)
	require.NoError(t, err)
	require.Equal(t, "2026-06-08", out)

	out, err = AddDays("2026-03-01", -1)
	require.NoError(t, err)
	require.Equal(t, "2026-02-28", out)
}

func TestEWMARecencyWeighted(t *testing.T) {
	// alpha=0.5: e=10; e=0.5*20+0.5*10=15; e=0.5*30+0.5*15=22.5
	require.InDelta(t, 22.5, ewma([]int{10, 20, 30}, 0.5), 1e-9)
	require.Equal(t, 0.0, ewma(nil, 0.5))
}

func TestRelGrowthShrinksWithSize(t *testing.T) {
	small := relGrowth(100, 90)    // /log10(100)=2 -> 50
	big := relGrowth(100, 99990)   // /log10(100000)=5 -> 20
	require.InDelta(t, 50, small, 1e-9)
	require.InDelta(t, 20, big, 1e-9)
	require.Greater(t, small, big)
}

func TestWindowSignalsSplitsCurrentAndPrevWindows(t *testing.T) {
	in := RepoInput{
		RepositoryID: 1, Stars: 1000,
		Deltas: []DayDelta{
			{Date: "2026-06-07", StarDelta: 5},  // prev window (06-07..06-08)
			{Date: "2026-06-08", StarDelta: 7},  // prev window
			{Date: "2026-06-09", StarDelta: 10}, // current window (06-09..06-10)
			{Date: "2026-06-10", StarDelta: 20}, // current window
		},
	}
	wd, accel, ew, err := windowSignals(in, "2026-06-10", 2, 0.5)
	require.NoError(t, err)
	require.Equal(t, 30, wd)               // 10+20
	require.InDelta(t, 30-12, accel, 1e-9) // current 30 - prev 12
	require.InDelta(t, 0.5*20+0.5*10, ew, 1e-9) // ewma over [10,20] asc
}

func TestNormalizeMinMax(t *testing.T) {
	out := normalize([]float64{10, 20, 30})
	require.InDelta(t, 0.0, out[0], 1e-9)
	require.InDelta(t, 0.5, out[1], 1e-9)
	require.InDelta(t, 1.0, out[2], 1e-9)

	// 全相等 -> 全 0(避免除零)
	out = normalize([]float64{5, 5, 5})
	require.Equal(t, []float64{0, 0, 0}, out)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/scoring/ -run "TestAddDays|TestEWMA|TestRelGrowth|TestWindowSignals|TestNormalize" -v`
Expected: FAIL — 包/标识符未定义。

- [ ] **Step 3: 实现配置与信号函数**

`internal/scoring/scoring.go`:
```go
package scoring

import (
	"math"
	"time"
)

// Weights 是各信号在综合分里的权重。
type Weights struct {
	EWMA      float64
	Accel     float64
	Window    float64
	RelGrowth float64
}

// Config 是评分的全部可调参数。
type Config struct {
	Weights    Weights
	Alpha      float64        // EWMA 平滑因子
	MinStars   int            // 候选仓库的最低 star 门槛
	TopN       int            // 每个周期保留的榜单条数
	PeriodDays map[string]int // 周期 -> 窗口天数
}

// DefaultConfig 给出可上线的默认参数(可经环境/配置覆盖以离线调参)。
func DefaultConfig() Config {
	return Config{
		Weights:    Weights{EWMA: 0.3, Accel: 0.2, Window: 0.3, RelGrowth: 0.2},
		Alpha:      0.5,
		MinStars:   50,
		TopN:       200,
		PeriodDays: map[string]int{"daily": 1, "weekly": 7, "monthly": 30},
	}
}

// DayDelta 是某天的 star 增量。
type DayDelta struct {
	Date      string // YYYY-MM-DD
	StarDelta int
}

// RepoInput 是单仓库的评分输入。Deltas 必须按日期升序,且覆盖至少最近 2*window 天。
type RepoInput struct {
	RepositoryID int64
	Language     string
	Stars        int
	Deltas       []DayDelta
}

// Scored 是单仓库在某周期的评分结果。
type Scored struct {
	RepositoryID int64
	Score        float64
	WindowDelta  int
	Language     string
}

// AddDays 返回 date 偏移 n 天后的 YYYY-MM-DD(n 可为负)。
func AddDays(date string, n int) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, n).Format("2006-01-02"), nil
}

// ewma 对按时间升序的 deltas 做指数加权移动平均;空序列返回 0。
func ewma(deltas []int, alpha float64) float64 {
	if len(deltas) == 0 {
		return 0
	}
	e := float64(deltas[0])
	for _, d := range deltas[1:] {
		e = alpha*float64(d) + (1-alpha)*e
	}
	return e
}

// relGrowth 用 log10 压缩仓库规模,让中小仓库有机会。
func relGrowth(windowDelta, stars int) float64 {
	return float64(windowDelta) / math.Log10(float64(stars)+10)
}

// windowSignals 把仓库的 deltas 按截止日 D 与窗口 window 拆成当前窗口与前一等长窗口,
// 返回当前窗口增量和、加速度(当前和-前窗和)、当前窗口 EWMA。
func windowSignals(in RepoInput, asOf string, window int, alpha float64) (windowDelta int, accel float64, ew float64, err error) {
	curLo, err := AddDays(asOf, -(window - 1))
	if err != nil {
		return 0, 0, 0, err
	}
	prevHi, err := AddDays(asOf, -window)
	if err != nil {
		return 0, 0, 0, err
	}
	prevLo, err := AddDays(asOf, -(2*window - 1))
	if err != nil {
		return 0, 0, 0, err
	}
	var curDeltas []int
	curSum, prevSum := 0, 0
	for _, d := range in.Deltas {
		switch {
		case d.Date >= curLo && d.Date <= asOf:
			curSum += d.StarDelta
			curDeltas = append(curDeltas, d.StarDelta)
		case d.Date >= prevLo && d.Date <= prevHi:
			prevSum += d.StarDelta
		}
	}
	return curSum, float64(curSum - prevSum), ewma(curDeltas, alpha), nil
}

// normalize 把切片 min-max 归一到 [0,1];全相等(含空)返回全 0。
func normalize(xs []float64) []float64 {
	out := make([]float64, len(xs))
	if len(xs) == 0 {
		return out
	}
	min, max := xs[0], xs[0]
	for _, x := range xs {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}
	if max == min {
		return out
	}
	for i, x := range xs {
		out[i] = (x - min) / (max - min)
	}
	return out
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/scoring/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/scoring/scoring.go internal/scoring/signals_test.go
git commit -m "$(printf 'feat(scoring): config and pure momentum signal functions\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 4: RankPeriod(候选过滤 + 归一 + 加权 + 排序 + TopN)

**Files:**
- Create: `internal/scoring/rank.go`
- Test: `internal/scoring/rank_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/scoring/rank_test.go`:
```go
package scoring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func dailyInput(id int64, lang string, stars, delta int, date string) RepoInput {
	return RepoInput{
		RepositoryID: id, Language: lang, Stars: stars,
		Deltas: []DayDelta{{Date: date, StarDelta: delta}},
	}
}

func TestRankPeriodOrdersByScoreDesc(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TopN = 100
	// window=1(daily),截止 2026-06-10,三个同 star 仓库,增量 200/100/50
	inputs := []RepoInput{
		dailyInput(2, "Go", 1000, 100, "2026-06-10"),
		dailyInput(3, "Rust", 1000, 50, "2026-06-10"),
		dailyInput(1, "Go", 1000, 200, "2026-06-10"),
	}
	scored, err := RankPeriod("2026-06-10", 1, inputs, cfg)
	require.NoError(t, err)
	require.Len(t, scored, 3)
	// 同 star 时分数随增量单调 → 排名 200 > 100 > 50
	require.Equal(t, int64(1), scored[0].RepositoryID)
	require.Equal(t, int64(2), scored[1].RepositoryID)
	require.Equal(t, int64(3), scored[2].RepositoryID)
	require.Equal(t, 200, scored[0].WindowDelta)
	require.Equal(t, "Go", scored[0].Language)
}

func TestRankPeriodFiltersLowStarsAndNonGrowth(t *testing.T) {
	cfg := DefaultConfig() // MinStars=50
	inputs := []RepoInput{
		dailyInput(1, "Go", 1000, 100, "2026-06-10"), // 候选
		dailyInput(2, "Go", 10, 100, "2026-06-10"),   // stars < MinStars → 排除
		dailyInput(3, "Go", 1000, 0, "2026-06-10"),   // windowDelta=0 → 排除
		dailyInput(4, "Go", 1000, -5, "2026-06-10"),  // 净下降 → 排除
	}
	scored, err := RankPeriod("2026-06-10", 1, inputs, cfg)
	require.NoError(t, err)
	require.Len(t, scored, 1)
	require.Equal(t, int64(1), scored[0].RepositoryID)
}

func TestRankPeriodAppliesTopN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TopN = 2
	inputs := []RepoInput{
		dailyInput(1, "Go", 1000, 200, "2026-06-10"),
		dailyInput(2, "Go", 1000, 100, "2026-06-10"),
		dailyInput(3, "Go", 1000, 50, "2026-06-10"),
	}
	scored, err := RankPeriod("2026-06-10", 1, inputs, cfg)
	require.NoError(t, err)
	require.Len(t, scored, 2)
	require.Equal(t, int64(1), scored[0].RepositoryID)
	require.Equal(t, int64(2), scored[1].RepositoryID)
}

func TestRankPeriodEmptyCohort(t *testing.T) {
	scored, err := RankPeriod("2026-06-10", 1, nil, DefaultConfig())
	require.NoError(t, err)
	require.Empty(t, scored)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/scoring/ -run TestRankPeriod -v`
Expected: FAIL — `undefined: RankPeriod`。

- [ ] **Step 3: 实现 RankPeriod**

`internal/scoring/rank.go`:
```go
package scoring

import "sort"

type candidate struct {
	id          int64
	lang        string
	windowDelta int
	ewma        float64
	accel       float64
	rel         float64
}

// RankPeriod 对截止日 asOf、窗口 window 的所有候选仓库算综合分并排名:
// 过滤(stars>=MinStars 且 windowDelta>0)→ 各信号 cohort 内 min-max 归一 → 加权求和 →
// 按分数降序(同分按 repo id 升序稳定)→ 截断 TopN。
func RankPeriod(asOf string, window int, inputs []RepoInput, cfg Config) ([]Scored, error) {
	var cohort []candidate
	for _, in := range inputs {
		if in.Stars < cfg.MinStars {
			continue
		}
		wd, accel, ew, err := windowSignals(in, asOf, window, cfg.Alpha)
		if err != nil {
			return nil, err
		}
		if wd <= 0 {
			continue
		}
		cohort = append(cohort, candidate{
			id: in.RepositoryID, lang: in.Language,
			windowDelta: wd, ewma: ew, accel: accel,
			rel: relGrowth(wd, in.Stars),
		})
	}
	if len(cohort) == 0 {
		return nil, nil
	}

	ewmaVals := make([]float64, len(cohort))
	accelVals := make([]float64, len(cohort))
	windowVals := make([]float64, len(cohort))
	relVals := make([]float64, len(cohort))
	for i, c := range cohort {
		ewmaVals[i] = c.ewma
		accelVals[i] = c.accel
		windowVals[i] = float64(c.windowDelta)
		relVals[i] = c.rel
	}
	ewmaN := normalize(ewmaVals)
	accelN := normalize(accelVals)
	windowN := normalize(windowVals)
	relN := normalize(relVals)

	scored := make([]Scored, len(cohort))
	for i, c := range cohort {
		score := cfg.Weights.EWMA*ewmaN[i] +
			cfg.Weights.Accel*accelN[i] +
			cfg.Weights.Window*windowN[i] +
			cfg.Weights.RelGrowth*relN[i]
		scored[i] = Scored{
			RepositoryID: c.id, Score: score,
			WindowDelta: c.windowDelta, Language: c.lang,
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].RepositoryID < scored[j].RepositoryID
	})
	if cfg.TopN > 0 && len(scored) > cfg.TopN {
		scored = scored[:cfg.TopN]
	}
	return scored, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/scoring/...`
Expected: PASS（含 Task 3 的信号测试)。

- [ ] **Step 5: 提交**

```bash
git add internal/scoring/rank.go internal/scoring/rank_test.go
git commit -m "$(printf 'feat(scoring): RankPeriod with cohort normalization and top-N\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 5: RunScoring 作业(加载 → 构建输入 → 逐周期排名 → 物化)

**Files:**
- Create: `internal/ingest/scoring.go`
- Test: `internal/ingest/scoring_test.go`

- [ ] **Step 1: 写失败的测试**

`internal/ingest/scoring_test.go`:
```go
package ingest

import (
	"context"
	"testing"

	"github.com/meirongdev/trends/internal/scoring"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// seedRepo 建一个仓库并设定其当前 star 数 + 当天快照增量。
func seedRepo(t *testing.T, db *store.DB, githubID int64, node, fullName string, stars, delta int, date string) int64 {
	t.Helper()
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: githubID, NodeID: node, FullName: fullName, Owner: "a", Name: node, HTMLURL: "u", Language: "Go",
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(githubID, stars, 0, 0, 0, date+"T00:00:00Z"))
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id, Date: date, Stars: stars, StarDelta: delta}))
	return id
}

func TestRunScoringMaterializesDailyRankings(t *testing.T) {
	db := newTestDB(t)
	idA := seedRepo(t, db, 1, "RA", "a/A", 1000, 200, "2026-06-10")
	idB := seedRepo(t, db, 2, "RB", "a/B", 1000, 100, "2026-06-10")
	_ = seedRepo(t, db, 3, "RC", "a/C", 10, 100, "2026-06-10") // stars < MinStars → 不上榜

	cfg := scoring.DefaultConfig()
	require.NoError(t, RunScoring(context.Background(), db, "2026-06-10", cfg))

	// daily 榜:A(增量200) 第1,B(增量100) 第2,C 被过滤
	type row struct {
		repoID    int64
		rank      int
		starDelta int
	}
	rows, err := db.SQL().Query(
		`SELECT repository_id, rank, star_delta FROM trending_rankings WHERE period='daily' AND period_date='2026-06-10' ORDER BY rank`)
	require.NoError(t, err)
	defer rows.Close()
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.repoID, &r.rank, &r.starDelta))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []row{{idA, 1, 200}, {idB, 2, 100}}, got)

	// weekly / monthly 也应被物化(同样两条)
	var weekly, monthly int
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings WHERE period='weekly' AND period_date='2026-06-10'`).Scan(&weekly))
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings WHERE period='monthly' AND period_date='2026-06-10'`).Scan(&monthly))
	require.Equal(t, 2, weekly)
	require.Equal(t, 2, monthly)
}

func TestRunScoringEmptyUniverseIsNoop(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, RunScoring(context.Background(), db, "2026-06-10", scoring.DefaultConfig()))
	var count int
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings`).Scan(&count))
	require.Equal(t, 0, count)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `GOPROXY=off go test ./internal/ingest/ -run TestRunScoring -v`
Expected: FAIL — `undefined: RunScoring`。

- [ ] **Step 3: 实现 RunScoring**

`internal/ingest/scoring.go`:
```go
package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/scoring"
	"github.com/meirongdev/trends/internal/store"
)

// RunScoring 基于截止日 asOf 的每日快照,为每个周期算分并物化 trending_rankings。
func RunScoring(ctx context.Context, db *store.DB, asOf string, cfg scoring.Config) error {
	meta, err := db.ActiveRepoMeta()
	if err != nil {
		return err
	}

	// 加载足够的历史:最大窗口的两倍(算 accel 需要前一个等长窗口)。
	maxWindow := 1
	for _, w := range cfg.PeriodDays {
		if w > maxWindow {
			maxWindow = w
		}
	}
	from, err := scoring.AddDays(asOf, -(2*maxWindow - 1))
	if err != nil {
		return err
	}
	deltaRows, err := db.DailyDeltasInRange(from, asOf)
	if err != nil {
		return err
	}

	// 按 repo 聚合(deltaRows 已按 repo_id, date 升序)。
	deltasByRepo := make(map[int64][]scoring.DayDelta, len(meta))
	for _, r := range deltaRows {
		deltasByRepo[r.RepositoryID] = append(deltasByRepo[r.RepositoryID],
			scoring.DayDelta{Date: r.Date, StarDelta: r.StarDelta})
	}

	inputs := make([]scoring.RepoInput, 0, len(meta))
	for id, m := range meta {
		inputs = append(inputs, scoring.RepoInput{
			RepositoryID: id, Language: m.Language, Stars: m.Stars,
			Deltas: deltasByRepo[id],
		})
	}

	for period, window := range cfg.PeriodDays {
		if err := ctx.Err(); err != nil {
			return err
		}
		scored, err := scoring.RankPeriod(asOf, window, inputs, cfg)
		if err != nil {
			return err
		}
		ranks := make([]store.Ranking, len(scored))
		for i, s := range scored {
			ranks[i] = store.Ranking{
				RepositoryID: s.RepositoryID, Rank: i + 1, Score: s.Score,
				StarDelta: s.WindowDelta, Language: s.Language,
			}
		}
		if err := db.ReplaceRankings(period, asOf, ranks); err != nil {
			return err
		}
	}
	slog.Info("scoring complete", "asOf", asOf, "repos", len(inputs))
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `GOPROXY=off go test ./internal/ingest/...`
Expected: PASS（含原有 ingest 测试)。

- [ ] **Step 5: 提交**

```bash
git add internal/ingest/scoring.go internal/ingest/scoring_test.go
git commit -m "$(printf 'feat(ingest): RunScoring materializes daily/weekly/monthly rankings\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 6: 接入调度(快照后链式评分)+ RUN_ONCE=score

**Files:**
- Modify: `cmd/trends/main.go`

- [ ] **Step 1: 改写 main.go**

把 `cmd/trends/main.go` 整体替换为下面内容。变化:新增 `runScoring` 闭包(对今天跑 `RunScoring`);快照成功后链式评分(让榜单反映刚采集的当天);新增 `RUN_ONCE=score`。

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
	runSnapshot := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		return ingest.RunSnapshot(ctx, db, gh, todayUTC(), 100)
	}
	runScoring := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return ingest.RunScoring(ctx, db, todayUTC(), scoreCfg)
	}

	// RUN_ONCE=discovery|snapshot|score 用于手动触发一次后退出;失败以非零码退出。
	switch os.Getenv("RUN_ONCE") {
	case "discovery":
		if err := runDiscovery(); err != nil {
			slog.Error("discovery run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "snapshot":
		if err := runSnapshot(); err != nil {
			slog.Error("snapshot run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "score":
		if err := runScoring(); err != nil {
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
		// 快照成功后链式评分,使榜单反映当天数据。
		scheduler.Job{Spec: cfg.SnapshotCron, Run: func() {
			if err := runSnapshot(); err != nil {
				slog.Error("snapshot job", "err", err)
				return
			}
			if err := runScoring(); err != nil {
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
	slog.Info("trends started", "discovery_cron", cfg.DiscoveryCron, "snapshot_cron", cfg.SnapshotCron)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")
}
```

- [ ] **Step 2: 编译 + 全量测试 + 冒烟**

Run:
```bash
GOPROXY=off go vet ./...
GOPROXY=off go build ./...
GOPROXY=off go test ./...
# 冒烟:空 DB 上 RUN_ONCE=score 应 exit 0、物化 0 条
rm -f /tmp/trends_m1a.db*; RUN_ONCE=score DB_PATH=/tmp/trends_m1a.db GOPROXY=off go run ./cmd/trends 2>&1 | tail -2; echo "exit=$?"; rm -f /tmp/trends_m1a.db*
```
Expected: vet/build/test 全过;冒烟日志出现 `scoring complete ... repos=0`,`exit=0`。

- [ ] **Step 3: 提交**

```bash
git add cmd/trends/main.go
git commit -m "$(printf 'feat: run scoring after each daily snapshot; RUN_ONCE=score\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Self-Review(已核对)

- **Spec 覆盖**:对应 spec §5(动量评分:ewma/accel/rel_growth、min-max 归一、可配权重)、§5.4(日/周/月窗口物化 Top-N)、§6(`trending_rankings` 表)。**已记录的有意推迟**:fork 分量、winsorize、decay、yearly、per-period 权重(均符合 spec「可配置/Phase 2」)。读取 API(spec §7)是 M1b,不在本计划。
- **占位符**:无 TBD/TODO;每个代码步骤均给出完整代码与命令。
- **类型一致性**:`store.Ranking{RepositoryID,Rank,Score,StarDelta,Language}`、`store.ReplaceRankings(period,periodDate,rows)`、`store.DeltaRow`、`store.DailyDeltasInRange(from,to)`、`store.RepoMeta`、`store.ActiveRepoMeta()`、`scoring.{Config,Weights,DefaultConfig,DayDelta,RepoInput,Scored,AddDays,RankPeriod}`、`ingest.RunScoring(ctx,db,asOf,cfg)` 在各任务间一致。`scoring.DefaultConfig().PeriodDays` 的键 `daily/weekly/monthly` 与测试断言一致。
- **数据流**:RunScoring 用 `ActiveRepoMeta`(语言+当前stars)+ `DailyDeltasInRange`(2*maxWindow 天)构 `RepoInput` → 逐周期 `RankPeriod` → `ReplaceRankings`。评分依赖 `repositories.stars`(由 M0 的 Snapshot 作业 `UpdateRepositoryMetrics` 维护),故调度上「快照后评分」顺序正确。
- **幂等**:`ReplaceRankings` 按 period+date 整组替换 → 同日重跑安全。

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-03-m1a-scoring-rankings.md`.
