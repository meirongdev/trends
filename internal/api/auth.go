package api

import (
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/meirongdev/trends/internal/auth"
	"github.com/meirongdev/trends/internal/store"
)

const (
	sessionCookieName = "trends_session"
	stateCookieName   = "trends_oauth_state"
	sessionTTL        = 30 * 24 * time.Hour
	stateTTL          = 10 * time.Minute
)

func (s *Server) setCookie(w http.ResponseWriter, name, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", HttpOnly: true,
		Secure: s.secureCookies, SameSite: http.SameSiteLaxMode,
		MaxAge: int(maxAge.Seconds()),
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true,
		Secure: s.secureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// currentUser 从 session cookie 解析当前登录用户。
func (s *Server) currentUser(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return store.User{}, false
	}
	u, ok, err := s.db.SessionUser(auth.HashToken(c.Value))
	if err != nil || !ok {
		return store.User{}, false
	}
	return u, true
}

// handleAuthLogin 生成 state、种短时 cookie,302 跳到 provider 授权页。
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providers[r.URL.Query().Get("provider")]
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "login not configured for this provider")
		return
	}
	state, err := auth.RandomToken(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.setCookie(w, stateCookieName, state, stateTTL)
	http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

// handleAuthCallback 校验 state、换 token、取身份、建会话,302 回 /submit。
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, ok := s.providers[q.Get("provider")]
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "login not configured for this provider")
		return
	}
	c, err := r.Cookie(stateCookieName)
	if err != nil || c.Value == "" || c.Value != q.Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	s.clearCookie(w, stateCookieName)

	tok, err := p.Exchange(r.Context(), q.Get("code"))
	if err != nil {
		slog.Warn("oauth exchange failed", "provider", p.Name(), "err", err)
		http.Redirect(w, r, "/submit?auth_error=1", http.StatusFound)
		return
	}
	id, err := p.FetchIdentity(r.Context(), tok)
	if err != nil {
		slog.Warn("oauth identity fetch failed", "provider", p.Name(), "err", err)
		http.Redirect(w, r, "/submit?auth_error=1", http.StatusFound)
		return
	}

	uid, err := s.db.UpsertUser(store.User{
		Provider: id.Provider, ProviderUserID: id.ProviderUserID,
		Login: id.Login, Email: id.Email, AvatarURL: id.AvatarURL,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	raw, err := auth.RandomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	expires := time.Now().UTC().Add(sessionTTL).Format(time.RFC3339)
	if err := s.db.CreateSession(auth.HashToken(raw), uid, expires); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	_, _ = s.db.DeleteExpiredSessions()
	s.setCookie(w, sessionCookieName, raw, sessionTTL)
	http.Redirect(w, r, "/submit", http.StatusFound)
}

// handleAuthMe 返回当前登录用户(或 null)与已配置的 provider 列表。
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	names := make([]string, 0, len(s.providers))
	for n := range s.providers {
		names = append(names, n)
	}
	sort.Strings(names)

	resp := map[string]any{"user": nil, "providers": names}
	if u, ok := s.currentUser(r); ok {
		resp["user"] = map[string]string{"login": u.Login, "avatar_url": u.AvatarURL, "provider": u.Provider}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleAuthLogout 删除会话并清除 cookie。
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		_ = s.db.DeleteSession(auth.HashToken(c.Value))
	}
	s.clearCookie(w, sessionCookieName)
	w.WriteHeader(http.StatusNoContent)
}
