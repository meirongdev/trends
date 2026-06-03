package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyDeltasInRangeOrdersByRepoThenDate(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-08", Stars: 100, StarDelta: 10}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-09", Stars: 130, StarDelta: 30}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-10", Stars: 150, StarDelta: 20}))

	rows, err := db.DailyDeltasInRange("2026-06-09", "2026-06-10")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "2026-06-09", rows[0].Date)
	require.Equal(t, 30, rows[0].StarDelta)
	require.Equal(t, "2026-06-10", rows[1].Date)
	require.Equal(t, 20, rows[1].StarDelta)
	require.Equal(t, id, rows[0].RepositoryID)
}

func TestActiveRepoMetaReturnsLanguageAndStars(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo()) // Language "Go", github_id 111
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(111, 1000, 50, 5, 9, "2026-06-10T00:00:00Z"))

	meta, err := db.ActiveRepoMeta()
	require.NoError(t, err)
	m, ok := meta[id]
	require.True(t, ok)
	require.Equal(t, "Go", m.Language)
	require.Equal(t, 1000, m.Stars)
	require.Equal(t, id, m.ID)
}

func TestDailyDeltasInRangeMultiRepoOrdering(t *testing.T) {
	db := newTestDB(t)
	id1, err := db.UpsertRepository(sampleRepo()) // github_id 111
	require.NoError(t, err)
	r2 := sampleRepo()
	r2.GitHubID = 222
	r2.FullName = "octocat/world"
	r2.NodeID = "R_node_222"
	id2, err := db.UpsertRepository(r2)
	require.NoError(t, err)

	// 乱序插入,验证按 repo_id 升序再按日期升序
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id2, Date: "2026-06-10", StarDelta: 1}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id1, Date: "2026-06-10", StarDelta: 2}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id1, Date: "2026-06-09", StarDelta: 3}))

	rows, err := db.DailyDeltasInRange("2026-06-09", "2026-06-10")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, id1, rows[0].RepositoryID)
	require.Equal(t, "2026-06-09", rows[0].Date)
	require.Equal(t, id1, rows[1].RepositoryID)
	require.Equal(t, "2026-06-10", rows[1].Date)
	require.Equal(t, id2, rows[2].RepositoryID)
	require.Equal(t, "2026-06-10", rows[2].Date)
}

func TestActiveRepoMetaExcludesInactive(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	_, err = db.SQL().Exec(`UPDATE repositories SET is_active=0 WHERE id=?`, id)
	require.NoError(t, err)

	meta, err := db.ActiveRepoMeta()
	require.NoError(t, err)
	_, ok := meta[id]
	require.False(t, ok) // 非活跃仓库不返回
	require.Empty(t, meta)
}
