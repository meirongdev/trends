package api

import "net/http"

type developerDTO struct {
	Login       string `json:"login"`
	Avatar      string `json:"avatar"`
	Appearances int    `json:"appearances"`
}

type developersResponseDTO struct {
	Period  string         `json:"period"`
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
	Total   int            `json:"total"`
	Items   []developerDTO `json:"items"`
}

func (s *Server) handleDevelopers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "daily"
	}
	if !validPeriods[period] {
		writeError(w, http.StatusBadRequest, "invalid period (use daily|weekly|monthly)")
		return
	}
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage := atoiDefault(q.Get("per_page"), 25)
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	total, err := s.db.CountDevelopers(period)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	devs, err := s.db.ListDevelopers(period, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]developerDTO, 0, len(devs))
	for _, dv := range devs {
		items = append(items, developerDTO{Login: dv.Login, Avatar: dv.Avatar, Appearances: dv.Appearances})
	}
	writeJSON(w, http.StatusOK, developersResponseDTO{
		Period: period, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
