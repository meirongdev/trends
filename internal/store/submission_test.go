package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInsertAndListPendingSubmissions(t *testing.T) {
	db := newTestDB(t)
	id, err := db.InsertSubmission("octocat/hello", "1.2.3.4")
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Len(t, pend, 1)
	require.Equal(t, "octocat/hello", pend[0].FullName)
	require.Equal(t, "pending", pend[0].Status)
	require.Equal(t, "1.2.3.4", pend[0].SubmittedIP)
}

func TestMarkSubmission(t *testing.T) {
	db := newTestDB(t)
	id, err := db.InsertSubmission("a/b", "")
	require.NoError(t, err)
	require.NoError(t, db.MarkSubmission(id, "accepted", ""))

	pend, err := db.ListPendingSubmissions(10)
	require.NoError(t, err)
	require.Empty(t, pend) // 已不再 pending
}
