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

	var weekly, monthly, yearly int
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings WHERE period='weekly' AND period_date='2026-06-10'`).Scan(&weekly))
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings WHERE period='monthly' AND period_date='2026-06-10'`).Scan(&monthly))
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings WHERE period='yearly' AND period_date='2026-06-10'`).Scan(&yearly))
	require.Equal(t, 2, weekly)
	require.Equal(t, 2, monthly)
	require.Equal(t, 2, yearly)
}

func TestRunScoringEmptyUniverseIsNoop(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, RunScoring(context.Background(), db, "2026-06-10", scoring.DefaultConfig()))
	var count int
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM trending_rankings`).Scan(&count))
	require.Equal(t, 0, count)
}

// 端到端验证:weekly 窗口对多天历史求和,且活跃但本期无快照的仓库被过滤。
func TestRunScoringWeeklySumsMultiDayHistoryAndFiltersUnsnapshotted(t *testing.T) {
	db := newTestDB(t)

	// repo A:本周窗口 [06-04..06-10] 内多天有增量,累计 60
	idA, err := db.UpsertRepository(store.Repository{
		GitHubID: 1, NodeID: "RA", FullName: "a/A", Owner: "a", Name: "A", HTMLURL: "u", Language: "Go",
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(1, 1000, 0, 0, 0, "2026-06-10T00:00:00Z"))
	for _, s := range []struct {
		date  string
		delta int
	}{{"2026-06-08", 10}, {"2026-06-09", 20}, {"2026-06-10", 30}} {
		require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: idA, Date: s.date, Stars: 1000, StarDelta: s.delta}))
	}

	// repo B:活跃但从未拍过快照 → 本期 windowDelta=0,应被过滤
	_, err = db.UpsertRepository(store.Repository{
		GitHubID: 2, NodeID: "RB", FullName: "a/B", Owner: "a", Name: "B", HTMLURL: "u", Language: "Go",
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(2, 1000, 0, 0, 0, "2026-06-10T00:00:00Z"))

	require.NoError(t, RunScoring(context.Background(), db, "2026-06-10", scoring.DefaultConfig()))

	var count int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT COUNT(*) FROM trending_rankings WHERE period='weekly' AND period_date='2026-06-10'`).Scan(&count))
	require.Equal(t, 1, count) // 只有 A 上榜,B 被过滤

	var repoID int64
	var starDelta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT repository_id, star_delta FROM trending_rankings WHERE period='weekly' AND period_date='2026-06-10' AND rank=1`).Scan(&repoID, &starDelta))
	require.Equal(t, idA, repoID)
	require.Equal(t, 60, starDelta) // weekly 窗口累计 10+20+30
}
