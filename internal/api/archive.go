package api

import "net/http"

type archiveEntryDTO struct {
	Repository    repositoryDTO `json:"repository"`
	Appearances   int           `json:"appearances"`
	BestRank      int           `json:"best_rank"`
	PeakStarDelta int           `json:"peak_star_delta"`
	FirstRanked   string        `json:"first_ranked"`
	LastRanked    string        `json:"last_ranked"`
}

type archiveResponseDTO struct {
	Period  string            `json:"period"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Total   int               `json:"total"`
	Items   []archiveEntryDTO `json:"items"`
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "daily"
	}
	if !validPeriods[period] {
		writeError(w, http.StatusBadRequest, "invalid period (use daily|weekly|monthly|yearly)")
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

	total, err := s.db.CountArchive(period)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	entries, err := s.db.ListArchive(period, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]archiveEntryDTO, 0, len(entries))
	for _, e := range entries {
		items = append(items, archiveEntryDTO{
			Repository: repositoryDTO{
				ID: e.Repo.ID, FullName: e.Repo.FullName, Owner: e.Repo.Owner, Name: e.Repo.Name,
				Description: e.Repo.Description, Language: e.Repo.Language, Stars: e.Repo.Stars,
				HTMLURL: e.Repo.HTMLURL, OwnerAvatar: e.Repo.OwnerAvatar,
			},
			Appearances: e.Appearances, BestRank: e.BestRank, PeakStarDelta: e.PeakStarDelta,
			FirstRanked: e.FirstRanked, LastRanked: e.LastRanked,
		})
	}
	writeJSON(w, http.StatusOK, archiveResponseDTO{
		Period: period, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
