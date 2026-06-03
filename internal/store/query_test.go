package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// seedRanked 建仓库、设其语言+stars,并把一条 daily 榜单追加进 (daily, date)。
func seedRanked(t *testing.T, db *DB, githubID int64, node, fullName, lang string, stars, rank, delta int, score float64, date string) int64 {
	t.Helper()
	id, err := db.UpsertRepository(Repository{
		GitHubID: githubID, NodeID: node, FullName: fullName, Owner: "a", Name: node, HTMLURL: "https://gh/" + fullName, Language: lang,
	})
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(githubID, stars, 0, 0, 0, date+"T00:00:00Z"))
	require.NoError(t, db.ReplaceRankings("daily", date, append(rankingsFor(t, db, "daily", date), Ranking{
		RepositoryID: id, Rank: rank, Score: score, StarDelta: delta, Language: lang,
	})))
	return id
}

// rankingsFor 读回某 period+date 现有榜单(便于增量追加),测试辅助。
func rankingsFor(t *testing.T, db *DB, period, date string) []Ranking {
	t.Helper()
	rows, err := db.SQL().Query(`SELECT repository_id, rank, score, star_delta, COALESCE(language,'') FROM trending_rankings WHERE period=? AND period_date=? ORDER BY rank`, period, date)
	require.NoError(t, err)
	defer rows.Close()
	var out []Ranking
	for rows.Next() {
		var r Ranking
		require.NoError(t, rows.Scan(&r.RepositoryID, &r.Rank, &r.Score, &r.StarDelta, &r.Language))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestLatestRankingDate(t *testing.T) {
	db := newTestDB(t)
	_, ok, err := db.LatestRankingDate("daily")
	require.NoError(t, err)
	require.False(t, ok) // 无榜单

	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-09", []Ranking{{RepositoryID: id, Rank: 1, Score: 1, StarDelta: 10, Language: "Go"}}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{{RepositoryID: id, Rank: 1, Score: 1, StarDelta: 20, Language: "Go"}}))

	date, ok, err := db.LatestRankingDate("daily")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "2026-06-10", date)
}

func TestListAndCountRankingsWithLanguageFilterAndPaging(t *testing.T) {
	db := newTestDB(t)
	idGo := seedRanked(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-10")
	_ = seedRanked(t, db, 2, "R2", "a/go2", "Go", 800, 2, 100, 0.5, "2026-06-10")
	_ = seedRanked(t, db, 3, "R3", "a/rust1", "Rust", 500, 3, 50, 0.3, "2026-06-10")

	total, err := db.CountRankings("daily", "2026-06-10", "")
	require.NoError(t, err)
	require.Equal(t, 3, total)
	all, err := db.ListRankings("daily", "2026-06-10", "", 25, 0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, 1, all[0].Rank)
	require.Equal(t, idGo, all[0].Repo.ID)
	require.Equal(t, "a/go1", all[0].Repo.FullName)
	require.Equal(t, 200, all[0].StarDelta)

	goTotal, err := db.CountRankings("daily", "2026-06-10", "Go")
	require.NoError(t, err)
	require.Equal(t, 2, goTotal)
	goRows, err := db.ListRankings("daily", "2026-06-10", "Go", 25, 0)
	require.NoError(t, err)
	require.Len(t, goRows, 2)
	require.Equal(t, "Go", goRows[0].Repo.Language) // 确认过滤确实只返回 Go
	require.Equal(t, "Go", goRows[1].Repo.Language)

	page2, err := db.ListRankings("daily", "2026-06-10", "", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, 2, page2[0].Rank)
}

func TestRankingLanguageCounts(t *testing.T) {
	db := newTestDB(t)
	seedRanked(t, db, 1, "R1", "a/go1", "Go", 1000, 1, 200, 0.9, "2026-06-10")
	seedRanked(t, db, 2, "R2", "a/go2", "Go", 800, 2, 100, 0.5, "2026-06-10")
	seedRanked(t, db, 3, "R3", "a/rust1", "Rust", 500, 3, 50, 0.3, "2026-06-10")
	// 不同日期的榜单不应计入 2026-06-10
	seedRanked(t, db, 4, "R4", "a/go3", "Go", 700, 1, 90, 0.4, "2026-06-09")

	counts, err := db.RankingLanguageCounts("daily", "2026-06-10")
	require.NoError(t, err)
	require.Equal(t, []LanguageCount{{Language: "Go", Count: 2}, {Language: "Rust", Count: 1}}, counts)

	// 空 period+date → 空
	empty, err := db.RankingLanguageCounts("daily", "2020-01-01")
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestHealthInfo(t *testing.T) {
	db := newTestDB(t)
	h, err := db.HealthInfo()
	require.NoError(t, err)
	require.Equal(t, 0, h.ActiveRepos)
	require.Equal(t, "", h.LastSyncedAt)

	_, err = db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.UpdateRepositoryMetrics(111, 100, 0, 0, 0, "2026-06-10T00:00:00Z"))

	h, err = db.HealthInfo()
	require.NoError(t, err)
	require.Equal(t, 1, h.ActiveRepos)
	require.Equal(t, "2026-06-10T00:00:00Z", h.LastSyncedAt)
}
