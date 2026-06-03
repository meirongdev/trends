package api

import "net/http"

type languageDTO struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
}

func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	langs, err := s.db.LanguageCounts()
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
