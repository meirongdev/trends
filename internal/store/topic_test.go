package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func seedRepoTopics(t *testing.T, db *DB, gid int64, node, full, lang string, stars int, topics []string) int64 {
	t.Helper()
	id, err := db.UpsertRepository(Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, HTMLURL: "u", Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, 0, 0, 0, "2026-06-10T00:00:00Z"))
	require.NoError(t, db.SetRepositoryTopics(id, topics))
	return id
}

func TestSetAndGetRepositoryTopics(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.SetRepositoryTopics(id, []string{"ai", "cli"}))
	got, err := db.GetRepositoryTopics(id)
	require.NoError(t, err)
	require.Equal(t, []string{"ai", "cli"}, got)

	// 重设覆盖旧关联
	require.NoError(t, db.SetRepositoryTopics(id, []string{"web"}))
	got, err = db.GetRepositoryTopics(id)
	require.NoError(t, err)
	require.Equal(t, []string{"web"}, got)
}

func TestListTopicsAndRepositoriesByTopic(t *testing.T) {
	db := newTestDB(t)
	seedRepoTopics(t, db, 1, "RA", "a/a", "Go", 1000, []string{"ai", "cli"})
	seedRepoTopics(t, db, 2, "RB", "a/b", "Go", 2000, []string{"ai"})

	topics, err := db.ListTopics()
	require.NoError(t, err)
	require.Equal(t, []TopicCount{{Slug: "ai", Name: "ai", Count: 2}, {Slug: "cli", Name: "cli", Count: 1}}, topics)

	total, err := db.CountRepositoriesByTopic("ai")
	require.NoError(t, err)
	require.Equal(t, 2, total)

	repos, err := db.RepositoriesByTopic("ai", 25, 0)
	require.NoError(t, err)
	require.Len(t, repos, 2)
	require.Equal(t, "a/b", repos[0].FullName) // stars 2000 在前
	require.Equal(t, "a/a", repos[1].FullName)
}

// 统计口径(minStatsStars)把未达标(含 0 star)的仓库排除在话题列表/计数之外:
// 仅由 0 star 仓库支撑的话题不出现,且共享话题的计数不计入 0 star 仓库。
func TestTopicsExcludeBelowStatsThreshold(t *testing.T) {
	db := newTestDB(t)
	seedRepoTopics(t, db, 1, "RA", "a/a", "Go", 1000, []string{"shared"})          // 达标
	seedRepoTopics(t, db, 2, "RB", "a/b", "Go", 0, []string{"shared", "zeroonly"}) // 0 star,被排除

	topics, err := db.ListTopics()
	require.NoError(t, err)
	// "zeroonly" 只有 0 star 仓库支撑 → 整体消失;"shared" 计数只数达标仓库 → 1
	require.Equal(t, []TopicCount{{Slug: "shared", Name: "shared", Count: 1}}, topics)

	total, err := db.CountRepositoriesByTopic("shared")
	require.NoError(t, err)
	require.Equal(t, 1, total)

	repos, err := db.RepositoriesByTopic("shared", 25, 0)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "a/a", repos[0].FullName)

	zero, err := db.CountRepositoriesByTopic("zeroonly")
	require.NoError(t, err)
	require.Equal(t, 0, zero)
}
