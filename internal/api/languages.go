package api

import "net/http"

func (s *Server) handleLanguages(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
