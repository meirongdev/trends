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
