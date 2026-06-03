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
