package ingest

import (
	"context"
	"testing"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestRunSnapshotComputesDeltaAndWrites(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: 111, NodeID: "R_111", FullName: "a/x", Owner: "a", Name: "x", HTMLURL: "u",
	})
	require.NoError(t, err)

	// 预置昨天的快照:stars=400
	require.NoError(t, db.InsertSnapshot(store.Snapshot{
		RepositoryID: id, Date: "2026-06-02", Stars: 400,
	}))

	fc := &fakeClient{metrics: []github.RepoMetrics{
		{GitHubID: 111, Stars: 500, Forks: 40, OpenIssues: 7, Watchers: 9},
	}}

	err = RunSnapshot(context.Background(), db, fc, "2026-06-03", 100)
	require.NoError(t, err)

	// delta = 500 - 400 = 100
	stars, ok, err := db.LastStars(id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 500, stars)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, 500, got.Stars)                       // 冗余字段已更新
	require.Equal(t, "2026-06-03T", got.LastSyncedAt[:11]) // 已写同步时间(日期前缀)

	var delta int
	err = db.SQL().QueryRow(
		`SELECT star_delta FROM repository_snapshots WHERE repository_id=? AND snapshot_date=?`,
		id, "2026-06-03").Scan(&delta)
	require.NoError(t, err)
	require.Equal(t, 100, delta)
}

func TestRunSnapshotFirstTimeDeltaIsZero(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: 222, NodeID: "R_222", FullName: "a/y", Owner: "a", Name: "y", HTMLURL: "u",
	})
	require.NoError(t, err)

	fc := &fakeClient{metrics: []github.RepoMetrics{
		{GitHubID: 222, Stars: 50, Forks: 1, OpenIssues: 0, Watchers: 0},
	}}
	require.NoError(t, RunSnapshot(context.Background(), db, fc, "2026-06-03", 100))

	var delta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT star_delta FROM repository_snapshots WHERE repository_id=?`, id).Scan(&delta))
	require.Equal(t, 0, delta) // 无历史快照时 delta 记 0,避免把存量当增量
}

func TestRunSnapshotSameDayRerunPreservesDelta(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(store.Repository{
		GitHubID: 111, NodeID: "R_111", FullName: "a/x", Owner: "a", Name: "x", HTMLURL: "u",
	})
	require.NoError(t, err)
	require.NoError(t, db.InsertSnapshot(store.Snapshot{RepositoryID: id, Date: "2026-06-02", Stars: 400}))

	fc := &fakeClient{metrics: []github.RepoMetrics{{GitHubID: 111, Stars: 500}}}
	require.NoError(t, RunSnapshot(context.Background(), db, fc, "2026-06-03", 100))
	// 同日再跑一次:delta 必须仍是 100(相对 06-02 的 400),而不是被算成 0
	require.NoError(t, RunSnapshot(context.Background(), db, fc, "2026-06-03", 100))

	var delta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT star_delta FROM repository_snapshots WHERE repository_id=? AND snapshot_date=?`,
		id, "2026-06-03").Scan(&delta))
	require.Equal(t, 100, delta)
}

func TestRunSnapshotSkipsUnknownGitHubID(t *testing.T) {
	db := newTestDB(t)
	_, err := db.UpsertRepository(store.Repository{
		GitHubID: 111, NodeID: "R_111", FullName: "a/x", Owner: "a", Name: "x", HTMLURL: "u",
	})
	require.NoError(t, err)

	fc := &fakeClient{metrics: []github.RepoMetrics{{GitHubID: 999, Stars: 5}}} // 不在活跃集中
	require.NoError(t, RunSnapshot(context.Background(), db, fc, "2026-06-03", 100))

	var count int
	require.NoError(t, db.SQL().QueryRow(`SELECT COUNT(*) FROM repository_snapshots`).Scan(&count))
	require.Equal(t, 0, count) // 未知 github_id 被跳过,不写快照
}
