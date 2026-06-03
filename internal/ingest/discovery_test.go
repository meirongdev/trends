package ingest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

// fakeClient 实现 Discoverer(本任务)与 Fetcher(Task 8 追加),供作业测试,不触网。
type fakeClient struct {
	searchByQuery map[string][]store.Repository
	metrics       []github.RepoMetrics
}

func (f *fakeClient) SearchRepositories(_ context.Context, query string, page int) ([]store.Repository, error) {
	if page > 1 {
		return nil, nil // 只有一页
	}
	return f.searchByQuery[query], nil
}

func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunDiscoveryUpsertsAcrossQueries(t *testing.T) {
	db := newTestDB(t)
	fc := &fakeClient{searchByQuery: map[string][]store.Repository{
		"stars:100..200": {{GitHubID: 1, NodeID: "R1", FullName: "a/1", Owner: "a", Name: "1", HTMLURL: "u1"}},
		"stars:200..500": {{GitHubID: 2, NodeID: "R2", FullName: "a/2", Owner: "a", Name: "2", HTMLURL: "u2"}},
	}}

	n, err := RunDiscovery(context.Background(), db, fc, []string{"stars:100..200", "stars:200..500"}, 1)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	repos, err := db.ListActiveRepositories()
	require.NoError(t, err)
	require.Len(t, repos, 2)
}

// errClient 的 SearchRepositories 总是返回错误,用于测错误路径。
type errClient struct{}

func (errClient) SearchRepositories(context.Context, string, int) ([]store.Repository, error) {
	return nil, errors.New("boom")
}

// pagedClient 每页都返回一条唯一仓库(按页号变化),并记录调用次数,用于测 maxPages 上限。
type pagedClient struct{ calls int }

func (p *pagedClient) SearchRepositories(_ context.Context, _ string, page int) ([]store.Repository, error) {
	p.calls++
	id := int64(page)
	return []store.Repository{{
		GitHubID: id, NodeID: fmt.Sprintf("N%d", id),
		FullName: fmt.Sprintf("a/%d", id), Owner: "a", Name: fmt.Sprintf("%d", id), HTMLURL: "u",
	}}, nil
}

// breakClient 仅在第 1 页返回数据,之后返回空,用于测「空页 break」。
type breakClient struct{ calls int }

func (b *breakClient) SearchRepositories(_ context.Context, _ string, page int) ([]store.Repository, error) {
	b.calls++
	if page == 1 {
		return []store.Repository{{
			GitHubID: 7, NodeID: "N7", FullName: "a/7", Owner: "a", Name: "7", HTMLURL: "u",
		}}, nil
	}
	return nil, nil
}

func TestRunDiscoveryReturnsErrorFromSearch(t *testing.T) {
	db := newTestDB(t)
	_, err := RunDiscovery(context.Background(), db, errClient{}, []string{"q"}, 3)
	require.Error(t, err)
}

func TestRunDiscoveryRespectsMaxPages(t *testing.T) {
	db := newTestDB(t)
	pc := &pagedClient{}
	n, err := RunDiscovery(context.Background(), db, pc, []string{"q"}, 2)
	require.NoError(t, err)
	require.Equal(t, 2, n)        // 两页各一条
	require.Equal(t, 2, pc.calls) // 只翻了 2 页(尊重 maxPages 上限)
}

func TestRunDiscoveryBreaksOnEmptyPage(t *testing.T) {
	db := newTestDB(t)
	bc := &breakClient{}
	n, err := RunDiscovery(context.Background(), db, bc, []string{"q"}, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 2, bc.calls) // page1 有数据,page2 空→break,不会一路翻到 page10
}

func (f *fakeClient) FetchByNodeIDs(_ context.Context, _ []string) ([]github.RepoMetrics, error) {
	return f.metrics, nil
}
