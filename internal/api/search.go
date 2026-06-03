package api

import "net/http"

type searchResponseDTO struct {
	Query   string          `json:"query"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
	Total   int             `json:"total"`
	Items   []repositoryDTO `json:"items"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q is required")
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

	total, err := s.db.SearchRepositoriesCount(query, language)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	repos, err := s.db.SearchRepositoriesByText(query, language, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]repositoryDTO, 0, len(repos))
	for _, rp := range repos {
		items = append(items, repositoryDTO{
			ID: rp.ID, FullName: rp.FullName, Owner: rp.Owner, Name: rp.Name,
			Description: rp.Description, Language: rp.Language, Stars: rp.Stars,
			HTMLURL: rp.HTMLURL, OwnerAvatar: rp.OwnerAvatar,
		})
	}
	writeJSON(w, http.StatusOK, searchResponseDTO{
		Query: query, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
