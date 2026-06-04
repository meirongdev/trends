package api

import "net/http"

type topicCountDTO struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.db.ListTopics()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	out := make([]topicCountDTO, 0, len(topics))
	for _, tc := range topics {
		out = append(out, topicCountDTO{Slug: tc.Slug, Name: tc.Name, Count: tc.Count})
	}
	writeJSON(w, http.StatusOK, out)
}

type topicResponseDTO struct {
	Slug    string          `json:"slug"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
	Total   int             `json:"total"`
	Items   []repositoryDTO `json:"items"`
}

func (s *Server) handleTopic(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage := atoiDefault(q.Get("per_page"), 25)
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	total, err := s.db.CountRepositoriesByTopic(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	repos, err := s.db.RepositoriesByTopic(slug, perPage, (page-1)*perPage)
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
	writeJSON(w, http.StatusOK, topicResponseDTO{
		Slug: slug, Page: page, PerPage: perPage, Total: total, Items: items,
	})
}
