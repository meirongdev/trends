package api

import "net/http"

func (s *Server) handleTrending(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
