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

// DefaultConfig 给出可上线的默认参数(可经配置覆盖以离线调参)。
func DefaultConfig() Config {
	return Config{
		Weights:    Weights{EWMA: 0.3, Accel: 0.2, Window: 0.3, RelGrowth: 0.2},
		Alpha:      0.5,
		MinStars:   50,
		TopN:       200,
		PeriodDays: map[string]int{"daily": 1, "weekly": 7, "monthly": 30, "yearly": 365},
	}
}

// DayDelta 是某天的 star 增量。
type DayDelta struct {
	Date      string // YYYY-MM-DD
	StarDelta int
}

// RepoInput 是单仓库的评分输入。Deltas 必须按日期升序,覆盖至少最近 2*window 天。
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

// windowSignals 把仓库 deltas 按截止日 asOf 与窗口 window 拆成当前窗口与前一等长窗口,
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
	lo, hi := xs[0], xs[0]
	for _, x := range xs {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	if hi == lo {
		return out
	}
	for i, x := range xs {
		out[i] = (x - lo) / (hi - lo)
	}
	return out
}
