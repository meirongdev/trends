package api

import (
	"net/http"
	"strconv"
	"time"
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
	RepoCreatedAt string   `json:"repo_created_at"`
	BestDailyRank *int     `json:"best_daily_rank"`
	Topics        []string `json:"topics"`
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

	topics, err := s.db.GetRepositoryTopics(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if topics == nil {
		topics = []string{}
	}

	writeJSON(w, http.StatusOK, repositoryDetailDTO{
		ID: repo.ID, FullName: repo.FullName, Owner: repo.Owner, Name: repo.Name,
		Description: repo.Description, Language: repo.Language, Homepage: repo.Homepage,
		HTMLURL: repo.HTMLURL, OwnerAvatar: repo.OwnerAvatar,
		Stars: repo.Stars, Forks: repo.Forks, OpenIssues: repo.OpenIssues, Watchers: repo.Watchers,
		RepoCreatedAt: repo.RepoCreatedAt, BestDailyRank: bestRank, Topics: topics,
	})
}

type snapshotDTO struct {
	Date       string `json:"date"`
	Stars      int    `json:"stars"`
	Forks      int    `json:"forks"`
	OpenIssues int    `json:"open_issues"`
	Watchers   int    `json:"watchers"`
	StarDelta  int    `json:"star_delta"`
}

type snapshotsResponseDTO struct {
	RepositoryID int64         `json:"repository_id"`
	Snapshots    []snapshotDTO `json:"snapshots"`
}

func (s *Server) handleRepositorySnapshots(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRepoID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repository id")
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	for _, d := range []string{from, to} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				writeError(w, http.StatusBadRequest, "invalid date (use YYYY-MM-DD)")
				return
			}
		}
	}

	snaps, err := s.db.RepositorySnapshots(id, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]snapshotDTO, 0, len(snaps))
	for _, sn := range snaps {
		items = append(items, snapshotDTO{
			Date: sn.Date, Stars: sn.Stars, Forks: sn.Forks,
			OpenIssues: sn.OpenIssues, Watchers: sn.Watchers, StarDelta: sn.StarDelta,
		})
	}
	writeJSON(w, http.StatusOK, snapshotsResponseDTO{RepositoryID: id, Snapshots: items})
}

type rankingHistoryDTO struct {
	Period    string  `json:"period"`
	Date      string  `json:"date"`
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	StarDelta int     `json:"star_delta"`
}

type rankingsResponseDTO struct {
	RepositoryID int64               `json:"repository_id"`
	Rankings     []rankingHistoryDTO `json:"rankings"`
}

func (s *Server) handleRepositoryRankings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRepoID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repository id")
		return
	}
	hist, err := s.db.RepositoryRankingHistory(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	items := make([]rankingHistoryDTO, 0, len(hist))
	for _, h := range hist {
		items = append(items, rankingHistoryDTO{
			Period: h.Period, Date: h.Date, Rank: h.Rank, Score: h.Score, StarDelta: h.StarDelta,
		})
	}
	writeJSON(w, http.StatusOK, rankingsResponseDTO{RepositoryID: id, Rankings: items})
}
