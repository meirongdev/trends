package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// staticHandler 托管嵌入的前端构建(web 的 Vite 产物拷到 internal/api/dist)。
// 命中真实文件则返回该文件;否则(且非 /api、非 /healthz)回退 index.html,交给前端 react-router。
func staticHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 未注册的 API / 健康路径不应被当作 SPA 路由返回 index.html。
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := sub.Open(p); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA 回退:非文件路径返回 index.html。
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
