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
	from, err := scoring.AddDays(asOf, -(2*maxWindow-1))
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
