package api

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strconv"
)

var fullNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handleSubmit 接收 owner/repo 收录提交:要求登录 + 每用户限流 + 格式校验 + 落库 pending。
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required")
		return
	}
	if !s.submitLimiter.allow(strconv.FormatInt(u.ID, 10)) {
		writeError(w, http.StatusTooManyRequests, "too many submissions, try again later")
		return
	}

	var body struct {
		FullName string `json:"full_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !fullNameRe.MatchString(body.FullName) {
		writeError(w, http.StatusBadRequest, "full_name must be owner/repo")
		return
	}

	id, err := s.db.InsertSubmission(body.FullName, clientIP(r), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "pending"})
}
