package api

import (
	"net/http"
	"time"
)

type repositoryDTO struct {
	ID          int64  `json:"id"`
	FullName    string `json:"full_name"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	HTMLURL     string `json:"html_url"`
	OwnerAvatar string `json:"owner_avatar"`
}

type trendingItemDTO struct {
	Rank       int           `json:"rank"`
	Score      float64       `json:"score"`
	StarDelta  int           `json:"star_delta"`
	Repository repositoryDTO `json:"repository"`
}

type trendingResponseDTO struct {
	Period  string            `json:"period"`
	Date    string            `json:"date"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Total   int               `json:"total"`
	Items   []trendingItemDTO `json:"items"`
}

var validPeriods = map[string]bool{"daily": true, "weekly": true, "monthly": true}

func (s *Server) handleTrending(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "daily"
	}
	if !validPeriods[period] {
		writeError(w, http.StatusBadRequest, "invalid period (use daily|weekly|monthly)")
		return
	}
	language := q.Get("language")
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage := atoiDefault(q.Get("per_page"), 25)
	if perPage < 1 || perPage > 100 {
		perPage = 25
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
			writeJSON(w, http.StatusOK, trendingResponseDTO{
				Period: period, Page: page, PerPage: perPage, Total: 0, Items: []trendingItemDTO{},
			})
			return
		}
		date = latest
	}

	total, err := s.db.CountRankings(period, date, language)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	ranked, err := s.db.ListRankings(period, date, language, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	items := make([]trendingItemDTO, 0, len(ranked))
	for _, rr := range ranked {
		items = append(items, trendingItemDTO{
			Rank: rr.Rank, Score: rr.Score, StarDelta: rr.StarDelta,
			Repository: repositoryDTO{
				ID: rr.Repo.ID, FullName: rr.Repo.FullName, Owner: rr.Repo.Owner, Name: rr.Repo.Name,
				Description: rr.Repo.Description, Language: rr.Repo.Language, Stars: rr.Repo.Stars,
				HTMLURL: rr.Repo.HTMLURL, OwnerAvatar: rr.Repo.OwnerAvatar,
			},
		})
	}
	writeJSON(w, http.StatusOK, trendingResponseDTO{
		Period: period, Date: date, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
