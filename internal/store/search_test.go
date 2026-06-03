package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func mkRepo(t *testing.T, db *DB, gid int64, node, full, desc, lang string, stars int) {
	t.Helper()
	_, err := db.UpsertRepository(Repository{
		GitHubID: gid, NodeID: node, FullName: full, Owner: "a", Name: node, Description: desc, HTMLURL: "u", Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(gid, stars, 0, 0, 0, "2026-06-10T00:00:00Z"))
}

func TestSearchRepositoriesByTextMatchesNameAndDescription(t *testing.T) {
	db := newTestDB(t)
	mkRepo(t, db, 1, "R1", "vercel/next.js", "The React framework", "JavaScript", 1000)
	mkRepo(t, db, 2, "R2", "facebook/react", "A JavaScript library", "JavaScript", 5000)
	mkRepo(t, db, 3, "R3", "golang/go", "The Go language", "Go", 3000)

	// 名称或简介匹配
	total, err := db.SearchRepositoriesCount("react", "")
	require.NoError(t, err)
	require.Equal(t, 2, total) // next.js(desc 含 React)+ facebook/react(名称含 react)
	rows, err := db.SearchRepositoriesByText("react", "", 25, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "facebook/react", rows[0].FullName) // stars 降序:5000 在前

	// 语言过滤
	total, err = db.SearchRepositoriesCount("react", "JavaScript")
	require.NoError(t, err)
	require.Equal(t, 2, total)
	total, err = db.SearchRepositoriesCount("language", "Go")
	require.NoError(t, err)
	require.Equal(t, 1, total) // 只有 golang/go 的 desc 含 "language" 且语言 Go

	// 分页
	page2, err := db.SearchRepositoriesByText("react", "", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "vercel/next.js", page2[0].FullName) // stars 次高
}

func TestSearchExcludesInactive(t *testing.T) {
	db := newTestDB(t)
	mkRepo(t, db, 1, "R1", "a/react-x", "x", "Go", 100)
	_, err := db.SQL().Exec(`UPDATE repositories SET is_active=0 WHERE github_id=1`)
	require.NoError(t, err)

	total, err := db.SearchRepositoriesCount("react", "")
	require.NoError(t, err)
	require.Equal(t, 0, total) // 非活跃不计入
}
