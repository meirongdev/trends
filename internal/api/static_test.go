package api

import (
	"net/http"
	"testing"
)

func TestServesIndexAtRoot(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("missing content-type on /")
	}
}

func TestUnknownPathFallsBackToIndex(t *testing.T) {
	s, _ := newTestServer(t)
	// 前端客户端路由,非 API、非真实文件 → 回退 index.html(200)
	rec := doGET(t, s, "/repositories/123")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /repositories/123 = %d, want 200 (SPA fallback)", rec.Code)
	}
}

func TestUnknownAPIPathIsNotSPAFallback(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doGET(t, s, "/api/v1/nope")
	if rec.Code == http.StatusOK {
		t.Fatalf("unknown /api path should NOT 200 via SPA fallback, got %d", rec.Code)
	}
}
