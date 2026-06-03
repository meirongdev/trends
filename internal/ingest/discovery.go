package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/store"
)

// Discoverer 是 Discovery 作业依赖的 GitHub 能力子集(便于测试 mock)。
type Discoverer interface {
	SearchRepositories(ctx context.Context, query string, page int) ([]store.Repository, error)
}

// RunDiscovery 遍历每个查询、逐页拉取直到空页或到 maxPages,upsert 所有仓库。
// 返回成功 upsert 的仓库数。
//
// 失败语义为快速失败:任一 SearchRepositories 或 UpsertRepository 出错即返回
// 「已完成计数 + 错误」。对磁盘满/库锁等瞬时故障,整体重跑比吞掉错误更安全。
func RunDiscovery(ctx context.Context, db *store.DB, gh Discoverer, queries []string, maxPages int) (int, error) {
	count := 0
	for _, q := range queries {
		for page := 1; page <= maxPages; page++ {
			// 长时间的发现扫描应能被取消(main 用 context.WithTimeout 包裹本作业)。
			if err := ctx.Err(); err != nil {
				return count, err
			}
			repos, err := gh.SearchRepositories(ctx, q, page)
			if err != nil {
				return count, err
			}
			if len(repos) == 0 {
				break
			}
			for _, r := range repos {
				// 此处不需要 upsert 返回的内部 id。
				if _, err := db.UpsertRepository(r); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	slog.Info("discovery complete", "queries", len(queries), "upserted", count)
	return count, nil
}
