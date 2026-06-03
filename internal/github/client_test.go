package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchRepositoriesParsesItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/search/repositories", r.URL.Path)
		require.Equal(t, "stars:100..200", r.URL.Query().Get("q"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
		  "items": [
		    {"id": 111, "node_id": "R_111", "full_name": "octo/a", "name": "a",
		     "owner": {"login": "octo", "avatar_url": "https://av/1"},
		     "description": "d", "language": "Go", "homepage": "",
		     "html_url": "https://github.com/octo/a",
		     "stargazers_count": 150, "forks_count": 20, "open_issues_count": 3,
		     "archived": false, "created_at": "2024-01-01T00:00:00Z"}
		  ]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	repos, err := c.SearchRepositories(context.Background(), "stars:100..200", 1)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, int64(111), repos[0].GitHubID)
	require.Equal(t, "R_111", repos[0].NodeID)
	require.Equal(t, "octo/a", repos[0].FullName)
	require.Equal(t, "Go", repos[0].Language)
	require.Equal(t, "https://av/1", repos[0].OwnerAvatar)
}

func TestSearchRepositoriesNon200ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	_, err := c.SearchRepositories(context.Background(), "x", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "403")
}

func TestSearchRepositoriesSetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", []string{"tok1"})
	_, err := c.SearchRepositories(context.Background(), "x", 1)
	require.NoError(t, err)
	require.Equal(t, "bearer tok1", gotAuth)
}

func TestFetchByNodeIDsParsesMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/graphql", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
		  "data": {
		    "nodes": [
		      {"databaseId": 111, "stargazerCount": 500, "forkCount": 40,
		       "issues": {"totalCount": 7}, "watchers": {"totalCount": 9}},
		      null
		    ],
		    "rateLimit": {"remaining": 4999, "resetAt": "2026-06-03T01:00:00Z"}
		  }
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	metrics, err := c.FetchByNodeIDs(context.Background(), []string{"R_111", "R_dead"})
	require.NoError(t, err)
	require.Len(t, metrics, 1) // null 节点被跳过
	require.Equal(t, int64(111), metrics[0].GitHubID)
	require.Equal(t, 500, metrics[0].Stars)
	require.Equal(t, 40, metrics[0].Forks)
	require.Equal(t, 7, metrics[0].OpenIssues)
	require.Equal(t, 9, metrics[0].Watchers)
}

func TestFetchByNodeIDsPartialErrorsReturnsValidNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
		  "data": {"nodes": [
		    {"databaseId": 1, "stargazerCount": 10, "forkCount": 1, "issues": {"totalCount": 0}, "watchers": {"totalCount": 0}},
		    null
		  ]},
		  "errors": [{"message": "Could not resolve to a node with the global id of 'R_dead'"}]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	metrics, err := c.FetchByNodeIDs(context.Background(), []string{"R_1", "R_dead"})
	require.NoError(t, err) // 个别坏 id 不丢弃整批
	require.Len(t, metrics, 1)
	require.Equal(t, int64(1), metrics[0].GitHubID)
}

func TestFetchByNodeIDsAllErrorsReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data": {"nodes": []}, "errors": [{"message": "Bad query"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL+"/graphql", nil)
	_, err := c.FetchByNodeIDs(context.Background(), []string{"R_x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Bad query")
}

func TestFetchByNodeIDsRejectsTooManyIDs(t *testing.T) {
	c := NewClient("http://unused", "http://unused/graphql", nil)
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "R_x"
	}
	_, err := c.FetchByNodeIDs(context.Background(), ids)
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many")
}
