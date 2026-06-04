package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
)

// Identity 是从 provider 取回并归一后的用户身份。
type Identity struct {
	Provider       string
	ProviderUserID string
	Login          string
	Email          string
	AvatarURL      string
}

const (
	GithubAuthURL     = "https://github.com/login/oauth/authorize"
	GithubTokenURL    = "https://github.com/login/oauth/access_token"
	GithubUserInfoURL = "https://api.github.com/user"
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// ProviderSpec 描述一个可构造的 provider(endpoint 可注入以便测试)。
type ProviderSpec struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	Parse        func([]byte) (Identity, error)
}

// Provider 封装一个 OAuth2 provider 的授权/换 token/取身份。
type Provider struct {
	name        string
	oauth       *oauth2.Config
	userInfoURL string
	parse       func([]byte) (Identity, error)
	client      *http.Client
}

func NewProvider(s ProviderSpec) *Provider {
	return &Provider{
		name: s.Name,
		oauth: &oauth2.Config{
			ClientID:     s.ClientID,
			ClientSecret: s.ClientSecret,
			RedirectURL:  s.RedirectURL,
			Scopes:       s.Scopes,
			Endpoint:     oauth2.Endpoint{AuthURL: s.AuthURL, TokenURL: s.TokenURL},
		},
		userInfoURL: s.UserInfoURL,
		parse:       s.Parse,
		client:      http.DefaultClient,
	}
}

func (p *Provider) Name() string        { return p.name }
func (p *Provider) RedirectURL() string { return p.oauth.RedirectURL }

func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauth.Exchange(ctx, code)
}

// FetchIdentity 用 access token 调 userinfo,归一为 Identity。
func (p *Provider) FetchIdentity(ctx context.Context, tok *oauth2.Token) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("userinfo %s: status %d", p.name, resp.StatusCode)
	}
	return p.parse(body)
}

// GithubParse 解析 GitHub /user 响应。
func GithubParse(body []byte) (Identity, error) {
	var g struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		return Identity{}, err
	}
	return Identity{
		Provider: "github", ProviderUserID: strconv.FormatInt(g.ID, 10),
		Login: g.Login, Email: g.Email, AvatarURL: g.AvatarURL,
	}, nil
}

// GoogleParse 解析 Google OIDC userinfo 响应。
func GoogleParse(body []byte) (Identity, error) {
	var g struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		return Identity{}, err
	}
	login := g.Name
	if login == "" {
		login = g.Email
	}
	return Identity{
		Provider: "google", ProviderUserID: g.Sub,
		Login: login, Email: g.Email, AvatarURL: g.Picture,
	}, nil
}

// Config 是从应用配置构造 providers 所需的输入。
type Config struct {
	BaseURL            string
	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
}

// NewProviders 仅为配置齐全(id+secret 都有)的 provider 构造实例。
func NewProviders(c Config) map[string]*Provider {
	out := map[string]*Provider{}
	if c.GitHubClientID != "" && c.GitHubClientSecret != "" {
		out["github"] = NewProvider(ProviderSpec{
			Name: "github", ClientID: c.GitHubClientID, ClientSecret: c.GitHubClientSecret,
			RedirectURL: c.BaseURL + "/api/v1/auth/callback?provider=github",
			AuthURL:     GithubAuthURL, TokenURL: GithubTokenURL, UserInfoURL: GithubUserInfoURL,
			Scopes: []string{"read:user"}, Parse: GithubParse,
		})
	}
	if c.GoogleClientID != "" && c.GoogleClientSecret != "" {
		out["google"] = NewProvider(ProviderSpec{
			Name: "google", ClientID: c.GoogleClientID, ClientSecret: c.GoogleClientSecret,
			RedirectURL: c.BaseURL + "/api/v1/auth/callback?provider=google",
			AuthURL:     GoogleAuthURL, TokenURL: GoogleTokenURL, UserInfoURL: GoogleUserInfoURL,
			Scopes: []string{"openid", "email", "profile"}, Parse: GoogleParse,
		})
	}
	return out
}
