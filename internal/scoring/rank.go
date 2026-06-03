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
