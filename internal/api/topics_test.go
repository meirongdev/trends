package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func seedRepoWithTopics(t *testing.T, db *store.DB, gid int64, node, full string, stars int, topics []string) int64 {
	t.Helper()
	id := seedRepoWithMetrics(t, db, gid, node, full, "Go", stars, 0, 0)
	require.NoError(t, db.SetRepositoryTopics(id, topics))
	return id
}

func TestTopicsList(t *testing.T) {
	s, db := newTestServer(t)
	seedRepoWithTopics(t, db, 1, "R1", "a/a", 1000, []string{"ai", "cli"})
	seedRepoWithTopics(t, db, 2, "R2", "a/b", 2000, []string{"ai"})

	rec := doGET(t, s, "/api/v1/topics")
	require.Equal(t, http.StatusOK, rec.Code)
	var body []struct {
		Slug  string `json:"slug"`
		Count int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, "ai", body[0].Slug)
	require.Equal(t, 2, body[0].Count)
}

func TestTopicsEmptyIsArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/topics")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[]\n", rec.Body.String())
}

func TestTopicDetail(t *testing.T) {
	s, db := newTestServer(t)
	seedRepoWithTopics(t, db, 1, "R1", "a/a", 1000, []string{"ai"})
	seedRepoWithTopics(t, db, 2, "R2", "a/b", 2000, []string{"ai"})

	rec := doGET(t, s, "/api/v1/topics/ai")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Slug  string `json:"slug"`
		Total int    `json:"total"`
		Items []struct {
			FullName string `json:"full_name"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ai", body.Slug)
	require.Equal(t, 2, body.Total)
	require.Len(t, body.Items, 2)
	require.Equal(t, "a/b", body.Items[0].FullName) // stars 2000 在前
}

func TestRepositoryDetailIncludesTopics(t *testing.T) {
	s, db := newTestServer(t)
	seedRepoWithTopics(t, db, 1, "R1", "a/a", 1000, []string{"ai", "cli"})

	rec := doGET(t, s, "/api/v1/repositories/1")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Topics []string `json:"topics"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"ai", "cli"}, body.Topics)
}
