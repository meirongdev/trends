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
	inputs := []RepoInput{
		dailyInput(2, "Go", 1000, 100, "2026-06-10"),
		dailyInput(3, "Rust", 1000, 50, "2026-06-10"),
		dailyInput(1, "Go", 1000, 200, "2026-06-10"),
	}
	scored, err := RankPeriod("2026-06-10", 1, inputs, cfg)
	require.NoError(t, err)
	require.Len(t, scored, 3)
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

func TestRankPeriodTieBreaksByRepoID(t *testing.T) {
	cfg := DefaultConfig()
	// 两个完全相同的候选 → 同分;输入故意逆序,断言按 repo id 升序、与输入顺序无关
	inputs := []RepoInput{
		dailyInput(5, "Go", 1000, 100, "2026-06-10"),
		dailyInput(2, "Go", 1000, 100, "2026-06-10"),
	}
	scored, err := RankPeriod("2026-06-10", 1, inputs, cfg)
	require.NoError(t, err)
	require.Len(t, scored, 2)
	require.Equal(t, int64(2), scored[0].RepositoryID)
	require.Equal(t, int64(5), scored[1].RepositoryID)
}

func TestRankPeriodSingleCandidateScoredZeroButReturned(t *testing.T) {
	cfg := DefaultConfig()
	scored, err := RankPeriod("2026-06-10", 1, []RepoInput{
		dailyInput(1, "Go", 1000, 100, "2026-06-10"),
	}, cfg)
	require.NoError(t, err)
	require.Len(t, scored, 1)
	require.Equal(t, int64(1), scored[0].RepositoryID)
	require.Equal(t, 0.0, scored[0].Score) // 单候选 min-max 归一为 0
	require.Equal(t, 100, scored[0].WindowDelta)
}
