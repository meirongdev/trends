package ingest

import (
	"context"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// fakeFetcher 实现 RepoFetcher;不在 repos 表中的 fullName 视为 not found。
type fakeFetcher struct {
	repos map[string]store.Repository
}

func (f *fakeFetcher) FetchRepository(_ context.Context, fullName string) (store.Repository, bool, error) {
	r, ok := f.repos[fullName]
	return r, ok, nil
}

func TestRunSubmissionsAcceptsAndRejects(t *testing.T) {
	db := newTestDB(t)
	_, err := db.InsertSubmission("octo/good", "ip", 0)
	require.NoError(t, err)
	_, err = db.InsertSubmission("no/such", "ip", 0)
	require.NoError(t, err)

	ff := &fakeFetcher{repos: map[string]store.Repository{
		"octo/good": {GitHubID: 1, NodeID: "R1", FullName: "octo/good", Owner: "octo", Name: "good", HTMLURL: "u"},
	}}
	require.NoError(t, RunSubmissions(context.Background(), db, ff, 10))

	// good 已收录进 repositories
	got, err := db.GetRepositoryByGitHubID(1)
	require.NoError(t, err)
	require.Equal(t, "octo/good", got.FullName)

	// 两条提交都已处理(不再 pending)
	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Empty(t, pend)
}

func TestRunSubmissionsSyncsTopics(t *testing.T) {
	db := newTestDB(t)
	_, err := db.InsertSubmission("octo/good", "ip", 0)
	require.NoError(t, err)
	ff := &fakeFetcher{repos: map[string]store.Repository{
		"octo/good": {GitHubID: 1, NodeID: "R1", FullName: "octo/good", Owner: "octo", Name: "good", HTMLURL: "u", Topics: []string{"go"}},
	}}
	require.NoError(t, RunSubmissions(context.Background(), db, ff, 10))

	repo, err := db.GetRepositoryByGitHubID(1)
	require.NoError(t, err)
	topics, err := db.GetRepositoryTopics(repo.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"go"}, topics)
}
