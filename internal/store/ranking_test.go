package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceRankingsInsertsRows(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	err = db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 120, Language: "Go"},
	})
	require.NoError(t, err)

	var rank, delta int
	var score float64
	err = db.SQL().QueryRow(
		`SELECT rank, score, star_delta FROM trending_rankings WHERE period=? AND period_date=? AND repository_id=?`,
		"daily", "2026-06-10", id).Scan(&rank, &score, &delta)
	require.NoError(t, err)
	require.Equal(t, 1, rank)
	require.Equal(t, 0.9, score)
	require.Equal(t, 120, delta)
}

func TestReplaceRankingsIsIdempotentPerPeriodDate(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.5, StarDelta: 50, Language: "Go"},
	}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.8, StarDelta: 99, Language: "Go"},
	}))

	var count, delta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT COUNT(*), MAX(star_delta) FROM trending_rankings WHERE period=? AND period_date=?`,
		"daily", "2026-06-10").Scan(&count, &delta))
	require.Equal(t, 1, count)
	require.Equal(t, 99, delta)
}

func TestReplaceRankingsEmptyClearsAndInsertsNothing(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.5, StarDelta: 50, Language: "Go"},
	}))
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", nil))

	var count int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT COUNT(*) FROM trending_rankings WHERE period=? AND period_date=?`,
		"daily", "2026-06-10").Scan(&count))
	require.Equal(t, 0, count)
}

func TestReplaceRankingsScopedToPeriodAndDate(t *testing.T) {
	db := newTestDB(t)
	id, err := db.UpsertRepository(sampleRepo())
	require.NoError(t, err)

	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.5, StarDelta: 50, Language: "Go"},
	}))
	require.NoError(t, db.ReplaceRankings("weekly", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.7, StarDelta: 70, Language: "Go"},
	}))

	// 替换 daily 不应影响 weekly
	require.NoError(t, db.ReplaceRankings("daily", "2026-06-10", []Ranking{
		{RepositoryID: id, Rank: 1, Score: 0.9, StarDelta: 99, Language: "Go"},
	}))

	var weeklyDelta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT star_delta FROM trending_rankings WHERE period='weekly' AND period_date='2026-06-10'`).Scan(&weeklyDelta))
	require.Equal(t, 70, weeklyDelta)

	var dailyDelta int
	require.NoError(t, db.SQL().QueryRow(
		`SELECT star_delta FROM trending_rankings WHERE period='daily' AND period_date='2026-06-10'`).Scan(&dailyDelta))
	require.Equal(t, 99, dailyDelta)
}
