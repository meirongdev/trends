package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
)

// Fetcher 是 Snapshot 作业依赖的 GitHub 能力子集(返回 github.RepoMetrics 这一唯一类型)。
type Fetcher interface {
	FetchByNodeIDs(ctx context.Context, nodeIDs []string) ([]github.RepoMetrics, error)
}

// RunSnapshot 给所有活跃仓库拍当日快照:批量拉指标 -> 算 star_delta -> 写快照 + 更新冗余字段。
func RunSnapshot(ctx context.Context, db *store.DB, gh Fetcher, date string, batchSize int) error {
	repos, err := db.ListActiveRepositories()
	if err != nil {
		return err
	}
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 100
	}

	// 建立 github_id -> 仓库内部 id 的映射,并按 node_id 分批。
	idByGitHubID := make(map[int64]int64, len(repos))
	nodeIDs := make([]string, 0, len(repos))
	for _, r := range repos {
		idByGitHubID[r.GitHubID] = r.ID
		nodeIDs = append(nodeIDs, r.NodeID)
	}

	syncedAt := date + "T00:00:00Z"
	written := 0

	for start := 0; start < len(nodeIDs); start += batchSize {
		end := start + batchSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}
		batch := nodeIDs[start:end]

		metrics, err := gh.FetchByNodeIDs(ctx, batch)
		if err != nil {
			return err
		}

		for _, m := range metrics {
			repoID, ok := idByGitHubID[m.GitHubID]
			if !ok {
				continue
			}

			// 基线取「当天之前」最近一次快照,排除当天 → 同日重跑不会把 delta 算成 0。
			delta := 0
			if prev, has, err := db.StarsBefore(repoID, date); err != nil {
				return err
			} else if has {
				delta = m.Stars - prev
			}

			if err := db.InsertSnapshot(store.Snapshot{
				RepositoryID: repoID, Date: date,
				Stars: m.Stars, Forks: m.Forks, OpenIssues: m.OpenIssues, Watchers: m.Watchers,
				StarDelta: delta,
			}); err != nil {
				return err
			}
			if err := db.UpdateRepositoryMetrics(m.GitHubID, m.Stars, m.Forks, m.OpenIssues, m.Watchers, syncedAt); err != nil {
				return err
			}
			written++
		}
	}
	slog.Info("snapshot complete", "date", date, "repos", len(repos), "written", written)
	return nil
}
