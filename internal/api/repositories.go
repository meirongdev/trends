package api

import (
	"net/http"
	"strconv"
)

type repositoryDetailDTO struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	Homepage      string `json:"homepage"`
	HTMLURL       string `json:"html_url"`
	OwnerAvatar   string `json:"owner_avatar"`
	Stars         int    `json:"stars"`
	Forks         int    `json:"forks"`
	OpenIssues    int    `json:"open_issues"`
	Watchers      int    `json:"watchers"`
	RepoCreatedAt string `json:"repo_created_at"`
	BestDailyRank *int   `json:"best_daily_rank"`
}

// parseRepoID 从路径取出 {id} 并解析为正整数;失败返回 ok=false(调用方回 400)。
func parseRepoID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (s *Server) handleRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRepoID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repository id")
		return
	}
	repo, found, err := s.db.GetRepositoryByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}

	var bestRank *int
	if br, has, err := s.db.BestDailyRank(id); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	} else if has {
		bestRank = &br
	}

	writeJSON(w, http.StatusOK, repositoryDetailDTO{
		ID: repo.ID, FullName: repo.FullName, Owner: repo.Owner, Name: repo.Name,
		Description: repo.Description, Language: repo.Language, Homepage: repo.Homepage,
		HTMLURL: repo.HTMLURL, OwnerAvatar: repo.OwnerAvatar,
		Stars: repo.Stars, Forks: repo.Forks, OpenIssues: repo.OpenIssues, Watchers: repo.Watchers,
		RepoCreatedAt: repo.RepoCreatedAt, BestDailyRank: bestRank,
	})
}
