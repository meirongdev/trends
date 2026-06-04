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
	small := relGrowth(100, 90)  // /log10(100)=2 -> 50
	big := relGrowth(100, 99990) // /log10(100000)=5 -> 20
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
	require.Equal(t, 30, wd)                    // 10+20
	require.InDelta(t, 30-12, accel, 1e-9)      // current 30 - prev 12
	require.InDelta(t, 0.5*20+0.5*10, ew, 1e-9) // ewma over [10,20] asc
}

func TestWindowSignalsDailyWindowIsSingleDay(t *testing.T) {
	// window=1(daily):当前窗口=[asOf,asOf],前窗=[asOf-1,asOf-1]
	in := RepoInput{
		RepositoryID: 1, Stars: 1000,
		Deltas: []DayDelta{
			{Date: "2026-06-09", StarDelta: 8},  // 前窗
			{Date: "2026-06-10", StarDelta: 20}, // 当前窗
		},
	}
	wd, accel, ew, err := windowSignals(in, "2026-06-10", 1, 0.5)
	require.NoError(t, err)
	require.Equal(t, 20, wd)              // 只有当天
	require.InDelta(t, 20-8, accel, 1e-9) // 当天 20 - 前一天 8
	require.InDelta(t, 20, ew, 1e-9)      // 单元素 ewma = 该值
}

func TestWindowSignalsEmptyDeltas(t *testing.T) {
	wd, accel, ew, err := windowSignals(RepoInput{RepositoryID: 1, Stars: 1000}, "2026-06-10", 7, 0.5)
	require.NoError(t, err)
	require.Equal(t, 0, wd)
	require.InDelta(t, 0, accel, 1e-9)
	require.InDelta(t, 0, ew, 1e-9)
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
