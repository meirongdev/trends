package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/meirongdev/trends/internal/store"
)

// Server 持有存储句柄与提交限流器,提供 HTTP 路由。
type Server struct {
	db            *store.DB
	submitLimiter *rateLimiter
}

func NewServer(db *store.DB) *Server {
	return &Server{db: db, submitLimiter: newRateLimiter(20, time.Hour)}
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
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	mux.HandleFunc("GET /api/v1/topics", s.handleTopics)
	mux.HandleFunc("GET /api/v1/topics/{slug}", s.handleTopic)
	mux.HandleFunc("GET /api/v1/developers", s.handleDevelopers)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("GET /api/v1/archive", s.handleArchive)
	mux.HandleFunc("GET /api/v1/repositories/{id}/badge.svg", s.handleBadge)
	mux.HandleFunc("POST /api/v1/submissions", s.handleSubmit)
	// 兜底:其余路径交给前端 SPA(静态文件或回退 index.html)。
	mux.Handle("/", staticHandler())
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
