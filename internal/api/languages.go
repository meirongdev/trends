package api

import (
	"net/http"
	"time"
)

type languageDTO struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
}

// handleLanguages 返回某 period+date 榜单上各语言的仓库数(默认 daily + 最新日期),用于筛选 Tab。
func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "daily"
	}
	if !validPeriods[period] {
		writeError(w, http.StatusBadRequest, "invalid period (use daily|weekly|monthly)")
		return
	}
	date := q.Get("date")
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeError(w, http.StatusBadRequest, "invalid date (use YYYY-MM-DD)")
			return
		}
	}
	if date == "" {
		latest, ok, err := s.db.LatestRankingDate(period)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
		if !ok {
			writeJSON(w, http.StatusOK, []languageDTO{})
			return
		}
		date = latest
	}

	langs, err := s.db.RankingLanguageCounts(period, date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	out := make([]languageDTO, 0, len(langs))
	for _, l := range langs {
		out = append(out, languageDTO{Language: l.Language, Count: l.Count})
	}
	writeJSON(w, http.StatusOK, out)
}
