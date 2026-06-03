package api

import "net/http"

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	h, err := s.db.HealthInfo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"last_synced_at": h.LastSyncedAt,
		"active_repos":   h.ActiveRepos,
	})
}
