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
