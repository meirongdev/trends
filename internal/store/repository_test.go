package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleRepo() Repository {
	return Repository{
		GitHubID:      111,
		NodeID:        "R_node_111",
		FullName:      "octocat/hello",
		Owner:         "octocat",
		Name:          "hello",
		Description:   "demo",
		Language:      "Go",
		HTMLURL:       "https://github.com/octocat/hello",
		OwnerAvatar:   "https://avatars/1",
		RepoCreatedAt: "2024-01-01T00:00:00Z",
	}
}

func TestUpsertInsertsThenUpdates(t *testing.T) {
	db := newTestDB(t)

	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	first, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.NotEmpty(t, first.FirstSeenAt)

	r := sampleRepo()
	r.Description = "updated"
	id2, err := db.UpsertRepository(r)
	require.NoError(t, err)
	require.Equal(t, id, id2)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, "updated", got.Description)
	require.True(t, got.IsActive)
	require.Equal(t, first.FirstSeenAt, got.FirstSeenAt)
}

func TestListActiveRepositories(t *testing.T) {
	db := newTestDB(t)
	_, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	repos, err := db.ListActiveRepositories()
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "R_node_111", repos[0].NodeID)
}

// 插入时即写入 stars 等指标(让新发现/提交的仓库立即带上真实星数,不必等下一次 snapshot);
// 但 ON CONFLICT 的更新分支不覆盖指标,保留 UpdateRepositoryMetrics(snapshot)维护的值。
func TestUpsertSeedsMetricsOnInsertButKeepsThemOnConflict(t *testing.T) {
	db := newTestDB(t)

	r := sampleRepo()
	r.Stars, r.Forks, r.OpenIssues = 42, 3, 7
	_, err := db.UpsertRepository(r)
	require.NoError(t, err)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, 42, got.Stars) // 未调用 UpdateRepositoryMetrics 也应有真实星数
	require.Equal(t, 3, got.Forks)
	require.Equal(t, 7, got.OpenIssues)

	// snapshot 回填权威指标
	require.NoError(t, db.UpdateRepositoryMetrics(111, 99, 9, 1, 5, "2026-06-10T00:00:00Z"))

	// 再次 upsert(如下一次 discovery 命中,携带较旧的搜索快照星数)不得覆盖 snapshot 值
	r.Stars, r.Forks, r.OpenIssues = 50, 4, 8
	_, err = db.UpsertRepository(r)
	require.NoError(t, err)

	got, err = db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, 99, got.Stars)
	require.Equal(t, 9, got.Forks)
	require.Equal(t, 1, got.OpenIssues)
	require.Equal(t, 5, got.Watchers)
}

func TestUpdateRepositoryMetrics(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	err = db.UpdateRepositoryMetrics(111, 500, 50, 5, 12, "2026-06-03T00:00:00Z")
	require.NoError(t, err)

	got, err := db.GetRepositoryByGitHubID(111)
	require.NoError(t, err)
	require.Equal(t, int64(id), int64(got.ID))
	require.Equal(t, 500, got.Stars)
	require.Equal(t, 50, got.Forks)
	require.Equal(t, 5, got.OpenIssues)
	require.Equal(t, 12, got.Watchers)
}
