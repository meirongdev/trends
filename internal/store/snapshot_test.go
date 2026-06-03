package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLastStarsReturnsFalseWhenNone(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	_, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestInsertSnapshotAndLastStars(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.InsertSnapshot(Snapshot{
		RepositoryID: id, Date: "2026-06-02", Stars: 100, Forks: 10, OpenIssues: 2, Watchers: 5, StarDelta: 0,
	}))

	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 100, stars)
}

func TestInsertSnapshotIsIdempotentPerDay(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	s := Snapshot{RepositoryID: id, Date: "2026-06-02", Stars: 100, Forks: 10, OpenIssues: 2, Watchers: 5}
	require.NoError(t, db.InsertSnapshot(s))
	s.Stars = 120
	require.NoError(t, db.InsertSnapshot(s))

	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 120, stars)
}

func TestLastStarsReturnsMostRecent(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-01", Stars: 50}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-03", Stars: 200}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-02", Stars: 100}))

	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 200, stars) // 2026-06-03 是最近的
}

func TestStarsBeforeExcludesGivenDate(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-02", Stars: 400}))
	require.NoError(t, db.InsertSnapshot(Snapshot{RepositoryID: id, Date: "2026-06-03", Stars: 500}))

	stars, ok, err := db.StarsBefore(id, "2026-06-03") // 只应看到 06-02
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 400, stars)

	_, ok, err = db.StarsBefore(id, "2026-06-02") // 没有更早的快照
	require.NoError(t, err)
	require.False(t, ok)
}
