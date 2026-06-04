package api

import "net/http"

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.db.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active_repos":        st.ActiveRepos,
		"total_snapshots":     st.TotalSnapshots,
		"languages":           st.Languages,
		"topics":              st.Topics,
		"developers":          st.Developers,
		"latest_ranking_date": st.LatestRankingDate,
		"last_synced_at":      st.LastSyncedAt,
	})
}
