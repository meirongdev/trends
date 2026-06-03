package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type searchResp struct {
	Query   string `json:"query"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Total   int    `json:"total"`
	Items   []struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		Stars    int    `json:"stars"`
	} `json:"items"`
}

func TestSearchEndpoint(t *testing.T) {
	s, db := newTestServer(t)
	seedRepoWithMetrics(t, db, 1, "R1", "vercel/next.js", "Go", 1000, 0, 0)
	seedRepoWithMetrics(t, db, 2, "R2", "facebook/react", "JavaScript", 5000, 0, 0)

	rec := doGET(t, s, "/api/v1/search?q=react")
	require.Equal(t, http.StatusOK, rec.Code)
	var body searchResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "react", body.Query)
	require.Equal(t, 1, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, "facebook/react", body.Items[0].FullName)
}

func TestSearchRequiresQuery(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/search")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearchEmptyResultIsEmptyArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/search?q=nomatch")
	require.Equal(t, http.StatusOK, rec.Code)
	var body searchResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Total)
	require.NotNil(t, body.Items)
	require.Len(t, body.Items, 0)
}
