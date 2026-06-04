package store

import (
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	ID            int64
	GitHubID      int64
	NodeID        string
	FullName      string
	Owner         string
	Name          string
	Description   string
	Language      string
	Homepage      string
	HTMLURL       string
	OwnerAvatar   string
	Stars         int
	Forks         int
	OpenIssues    int
	Watchers      int
	IsArchived    bool
	IsActive      bool
	RepoCreatedAt string
	FirstSeenAt   string
	LastSyncedAt  string

	// Topics 来自 GitHub API,非 repositories 列;由 ingest 同步进 repository_topics,
	// 从 DB 读取仓库时为空(详情页话题另经 GetRepositoryTopics 加载)。
	Topics []string
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// repoSelectColumns 是读取 Repository 的统一列清单(含对可空列的 COALESCE),
// Get 与 List 共用,避免列清单漂移。其顺序必须与 scanInto 的扫描顺序一致。
const repoSelectColumns = `id, github_id, node_id, full_name, owner, name,
       COALESCE(description,''), COALESCE(language,''), COALESCE(homepage,''),
       html_url, COALESCE(owner_avatar,''), stars, forks, open_issues, watchers,
       is_archived, is_active, COALESCE(repo_created_at,''), first_seen_at, COALESCE(last_synced_at,'')`

// UpsertRepository 按 github_id 插入或更新元数据,返回内部 id。
// first_seen_at 仅在插入时设置;stars/forks 等指标由 UpdateRepositoryMetrics 维护。
// 用后续 SELECT(按唯一键)取回 id:这比 LastInsertId 在 upsert 的 UPDATE 分支上更稳健、更可移植。
func (d *DB) UpsertRepository(r Repository) (int64, error) {
	_, err := d.db.Exec(`
INSERT INTO repositories
  (github_id, node_id, full_name, owner, name, description, language, homepage,
   html_url, owner_avatar, is_archived, repo_created_at, first_seen_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(github_id) DO UPDATE SET
  node_id        = excluded.node_id,
  full_name      = excluded.full_name,
  owner          = excluded.owner,
  name           = excluded.name,
  description    = excluded.description,
  language       = excluded.language,
  homepage       = excluded.homepage,
  html_url       = excluded.html_url,
  owner_avatar   = excluded.owner_avatar,
  is_archived    = excluded.is_archived,
  repo_created_at= excluded.repo_created_at
`,
		r.GitHubID, r.NodeID, r.FullName, r.Owner, r.Name, r.Description, r.Language, r.Homepage,
		r.HTMLURL, r.OwnerAvatar, b2i(r.IsArchived), r.RepoCreatedAt, nowUTC(),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.QueryRow(`SELECT id FROM repositories WHERE github_id=?`, r.GitHubID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (d *DB) GetRepositoryByGitHubID(githubID int64) (Repository, error) {
	return scanRepo(d.db.QueryRow(`SELECT `+repoSelectColumns+` FROM repositories WHERE github_id=?`, githubID))
}

func (d *DB) ListActiveRepositories() ([]Repository, error) {
	rows, err := d.db.Query(`SELECT ` + repoSelectColumns + ` FROM repositories WHERE is_active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Repository
	for rows.Next() {
		r, err := scanRepoRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRepositoryMetrics 仅更新可变指标与同步时间;若 github_id 不存在则返回错误,
// 避免静默的空更新掩盖上游 bug。
func (d *DB) UpdateRepositoryMetrics(githubID int64, stars, forks, openIssues, watchers int, syncedAt string) error {
	res, err := d.db.Exec(`
UPDATE repositories
SET stars=?, forks=?, open_issues=?, watchers=?, last_synced_at=?
WHERE github_id=?`,
		stars, forks, openIssues, watchers, syncedAt, githubID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("update metrics: repository github_id=%d not found", githubID)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRepo(row *sql.Row) (Repository, error)       { return scanInto(row) }
func scanRepoRows(rows *sql.Rows) (Repository, error) { return scanInto(rows) }

func scanInto(s rowScanner) (Repository, error) {
	var r Repository
	var archived, active int
	err := s.Scan(
		&r.ID, &r.GitHubID, &r.NodeID, &r.FullName, &r.Owner, &r.Name,
		&r.Description, &r.Language, &r.Homepage, &r.HTMLURL, &r.OwnerAvatar,
		&r.Stars, &r.Forks, &r.OpenIssues, &r.Watchers,
		&archived, &active, &r.RepoCreatedAt, &r.FirstSeenAt, &r.LastSyncedAt,
	)
	if err != nil {
		return Repository{}, err
	}
	r.IsArchived = archived == 1
	r.IsActive = active == 1
	return r, nil
}
