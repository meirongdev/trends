package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/meirongdev/trends/internal/store"
)

// Server 持有只读存储句柄,提供 HTTP 路由。
type Server struct {
	db *store.DB
}

func NewServer(db *store.DB) *Server {
	return &Server{db: db}
}

// Routes 构建并返回 HTTP 路由(Go 1.22+ 方法+路径模式)。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/v1/trending", s.handleTrending)
	mux.HandleFunc("GET /api/v1/languages", s.handleLanguages)
	mux.HandleFunc("GET /api/v1/repositories/{id}", s.handleRepository)
	mux.HandleFunc("GET /api/v1/repositories/{id}/snapshots", s.handleRepositorySnapshots)
	mux.HandleFunc("GET /api/v1/repositories/{id}/rankings", s.handleRepositoryRankings)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// atoiDefault 解析十进制整数,失败或空串返回 def。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
