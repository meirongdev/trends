package ingest

import (
	"context"
	"log/slog"

	"github.com/meirongdev/trends/internal/store"
)

// RepoFetcher 是处理提交所需的 GitHub 能力子集。
type RepoFetcher interface {
	FetchRepository(ctx context.Context, fullName string) (store.Repository, bool, error)
}

// RunSubmissions 处理 pending 提交:存在的仓库 upsert 进宇宙并标记 accepted,
// 不存在的标记 rejected;瞬时错误则中断(留待下次重试)。
func RunSubmissions(ctx context.Context, db *store.DB, gh RepoFetcher, limit int) error {
	subs, err := db.ListPendingSubmissions(limit)
	if err != nil {
		return err
	}
	accepted, rejected := 0, 0
	for _, sub := range subs {
		if err := ctx.Err(); err != nil {
			return err
		}
		repo, found, err := gh.FetchRepository(ctx, sub.FullName)
		if err != nil {
			return err
		}
		if !found {
			if err := db.MarkSubmission(sub.ID, "rejected", "repository not found"); err != nil {
				return err
			}
			rejected++
			continue
		}
		if _, err := db.UpsertRepository(repo); err != nil {
			return err
		}
		if err := db.MarkSubmission(sub.ID, "accepted", ""); err != nil {
			return err
		}
		accepted++
	}
	slog.Info("submissions processed", "accepted", accepted, "rejected", rejected, "pending_seen", len(subs))
	return nil
}
