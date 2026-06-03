package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/meirongdev/trends/internal/store"
)

// RepoMetrics 是 Snapshot 作业需要的可变指标(Task 6 的 GraphQL 拉取会填充它)。
type RepoMetrics struct {
	GitHubID   int64
	Stars      int
	Forks      int
	OpenIssues int
	Watchers   int
}

// Client 与 GitHub REST/GraphQL 通信。restBase 与 graphqlURL 在测试中指向 httptest。
type Client struct {
	httpClient *http.Client
	restBase   string
	graphqlURL string
	tokens     []string
	tokenIdx   uint32
}

func NewClient(restBase, graphqlURL string, tokens []string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		restBase:   restBase,
		graphqlURL: graphqlURL,
		tokens:     tokens,
	}
}

// nextToken 在多个 token 间轮询(0 起始);无 token 时返回空串(不带鉴权)。
func (c *Client) nextToken() string {
	if len(c.tokens) == 0 {
		return ""
	}
	i := atomic.AddUint32(&c.tokenIdx, 1) - 1
	return c.tokens[i%uint32(len(c.tokens))]
}

func (c *Client) auth(req *http.Request) {
	if tok := c.nextToken(); tok != "" {
		req.Header.Set("Authorization", "bearer "+tok)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
}

type searchResponse struct {
	Items []struct {
		ID       int64  `json:"id"`
		NodeID   string `json:"node_id"`
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Owner    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
		Description     string `json:"description"`
		Language        string `json:"language"`
		Homepage        string `json:"homepage"`
		HTMLURL         string `json:"html_url"`
		StargazersCount int    `json:"stargazers_count"`
		ForksCount      int    `json:"forks_count"`
		OpenIssuesCount int    `json:"open_issues_count"`
		Archived        bool   `json:"archived"`
		CreatedAt       string `json:"created_at"`
	} `json:"items"`
}

// SearchRepositories 调 REST /search/repositories,返回该页结果(映射为 store.Repository)。
func (c *Client) SearchRepositories(ctx context.Context, query string, page int) ([]store.Repository, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("sort", "stars")
	q.Set("order", "desc")
	q.Set("per_page", "100")
	q.Set("page", strconv.Itoa(page))

	endpoint := c.restBase + "/search/repositories?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.auth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return nil, fmt.Errorf("github search: status %d: %s", resp.StatusCode, body)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	repos := make([]store.Repository, 0, len(sr.Items))
	for _, it := range sr.Items {
		repos = append(repos, store.Repository{
			GitHubID:      it.ID,
			NodeID:        it.NodeID,
			FullName:      it.FullName,
			Owner:         it.Owner.Login,
			Name:          it.Name,
			Description:   it.Description,
			Language:      it.Language,
			Homepage:      it.Homepage,
			HTMLURL:       it.HTMLURL,
			OwnerAvatar:   it.Owner.AvatarURL,
			Stars:         it.StargazersCount,
			Forks:         it.ForksCount,
			OpenIssues:    it.OpenIssuesCount,
			IsArchived:    it.Archived,
			RepoCreatedAt: it.CreatedAt,
		})
	}
	return repos, nil
}

const repoMetricsQuery = `query($ids: [ID!]!) {
  nodes(ids: $ids) {
    ... on Repository {
      databaseId
      stargazerCount
      forkCount
      issues(states: OPEN) { totalCount }
      watchers { totalCount }
    }
  }
  rateLimit { remaining resetAt }
}`

type graphqlNode struct {
	DatabaseID     int64 `json:"databaseId"`
	StargazerCount int   `json:"stargazerCount"`
	ForkCount      int   `json:"forkCount"`
	Issues         struct {
		TotalCount int `json:"totalCount"`
	} `json:"issues"`
	Watchers struct {
		TotalCount int `json:"totalCount"`
	} `json:"watchers"`
}

type graphqlResponse struct {
	Data struct {
		Nodes []*graphqlNode `json:"nodes"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// FetchByNodeIDs 用 GraphQL nodes(ids:) 一次批量拉取多个仓库的指标。
// nodeIDs 数量必须 <= 100(GitHub 单请求上限,由调用方分批保证)。
// 返回中跳过已删除/无权限的 null 节点。
func (c *Client) FetchByNodeIDs(ctx context.Context, nodeIDs []string) ([]RepoMetrics, error) {
	if len(nodeIDs) > 100 {
		return nil, fmt.Errorf("github graphql: too many node ids: %d (max 100 per request)", len(nodeIDs))
	}

	body, err := json.Marshal(map[string]any{
		"query":     repoMetricsQuery,
		"variables": map[string]any{"ids": nodeIDs},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.auth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return nil, fmt.Errorf("github graphql: status %d: %s", resp.StatusCode, b)
	}

	var gr graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}

	out := make([]RepoMetrics, 0, len(gr.Data.Nodes))
	for _, n := range gr.Data.Nodes {
		if n == nil {
			continue
		}
		out = append(out, RepoMetrics{
			GitHubID:   n.DatabaseID,
			Stars:      n.StargazerCount,
			Forks:      n.ForkCount,
			OpenIssues: n.Issues.TotalCount,
			Watchers:   n.Watchers.TotalCount,
		})
	}

	// GitHub 可能同时返回 data 与 errors(例如批次中个别 node id 已失效)。
	// 只要拿到了有效节点,就告警并返回这些指标,而不是因个别坏 id 丢弃整批;
	// 仅当一个有效节点都没有时才视为整体失败。
	if len(gr.Errors) > 0 {
		if len(out) == 0 {
			return nil, fmt.Errorf("github graphql: %s", gr.Errors[0].Message)
		}
		slog.Warn("github graphql returned partial errors",
			"errors", len(gr.Errors), "first", gr.Errors[0].Message, "valid_nodes", len(out))
	}
	return out, nil
}
