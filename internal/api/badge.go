package api

import (
	"fmt"
	"net/http"

	"github.com/meirongdev/trends/internal/badge"
)

const (
	badgeLabel       = "trends"
	badgeColorRanked = "#3aa3e3" // 蓝
	badgeColorMuted  = "#9f9f9f" // 灰(未上榜 / 未知)
)

// handleBadge 返回仓库的趋势徽章 SVG。始终 200(未知 id 返回占位徽章,避免 README 破图)。
func (s *Server) handleBadge(w http.ResponseWriter, r *http.Request) {
	message, color := "n/a", badgeColorMuted

	if id, ok := parseRepoID(r); ok {
		_, found, err := s.db.GetRepositoryByID(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
		if found {
			rank, has, err := s.db.BestDailyRank(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db error")
				return
			}
			if has {
				message, color = fmt.Sprintf("rank #%d", rank), badgeColorRanked
			} else {
				message = "unranked"
			}
		}
	}

	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(badge.Render(badgeLabel, message, color)))
}
