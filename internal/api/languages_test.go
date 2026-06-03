package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/meirongdev/trends/internal/store"
	"github.com/stretchr/testify/require"
)

func TestLanguagesReturnsCounts(t *testing.T) {
	s, db := newTestServer(t)
	mk := func(gid int64, node, full, lang string) {
		_, err := db.UpsertRepository(store.Repository{GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, HTMLURL: "u", Language: lang})
		require.NoError(t, err)
	}
	mk(1, "R1", "a/1", "Go")
	mk(2, "R2", "a/2", "Go")
	mk(3, "R3", "a/3", "Rust")

	rec := doGET(t, s, "/api/v1/languages")
	require.Equal(t, http.StatusOK, rec.Code)
	var body []struct {
		Language string `json:"language"`
		Count    int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Equal(t, "Go", body[0].Language)
	require.Equal(t, 2, body[0].Count)
	require.Equal(t, "Rust", body[1].Language)
	require.Equal(t, 1, body[1].Count)
}

func TestLanguagesEmptyReturnsEmptyArray(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/languages")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[]\n", rec.Body.String()) // 空数组而非 null
}
