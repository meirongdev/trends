package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenRunsMigrations(t *testing.T) {
	db := newTestDB(t)

	var count int
	err := db.SQL().QueryRow(`SELECT COUNT(*) FROM repositories`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	err = db.SQL().QueryRow(`SELECT COUNT(*) FROM repository_snapshots`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db1, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, db1.Close())

	db2, err := Open(path)
	require.NoError(t, err)
	var count int
	require.NoError(t, db2.SQL().QueryRow(`SELECT COUNT(*) FROM repositories`).Scan(&count))
	require.Equal(t, 0, count)
	require.NoError(t, db2.Close())
}
