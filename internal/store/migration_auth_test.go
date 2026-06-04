package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration0005Tables(t *testing.T) {
	db := newTestDB(t)
	for _, tbl := range []string{"users", "sessions"} {
		var name string
		err := db.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		require.NoError(t, err, "table %s should exist", tbl)
		require.Equal(t, tbl, name)
	}
	var cnt int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('submissions') WHERE name='user_id'`).Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)
}
