export type Period = 'daily' | 'weekly' | 'monthly'

export interface Repository {
  id: number
  full_name: string
  owner: string
  name: string
  description: string
  language: string
  stars: number
  html_url: string
  owner_avatar: string
}

export interface TrendingItem {
  rank: number
  score: number
  star_delta: number
  repository: Repository
}

export interface TrendingResponse {
  period: Period
  date: string
  page: number
  per_page: number
  total: number
  items: TrendingItem[]
}

export interface LanguageCount {
  language: string
  count: number
}

export interface RepositoryDetail {
  id: number
  full_name: string
  owner: string
  name: string
  description: string
  language: string
  homepage: string
  html_url: string
  owner_avatar: string
  stars: number
  forks: number
  open_issues: number
  watchers: number
  repo_created_at: string
  best_daily_rank: number | null
  topics: string[]
}

export interface TopicCount {
  slug: string
  name: string
  count: number
}

export interface TopicResponse {
  slug: string
  page: number
  per_page: number
  total: number
  items: Repository[]
}

export interface Developer {
  login: string
  avatar: string
  appearances: number
}

export interface DevelopersResponse {
  period: Period
  page: number
  per_page: number
  total: number
  items: Developer[]
}

export interface Snapshot {
  date: string
  stars: number
  forks: number
  open_issues: number
  watchers: number
  star_delta: number
}

export interface SnapshotsResponse {
  repository_id: number
  snapshots: Snapshot[]
}

export interface RankingHistory {
  period: string
  date: string
  rank: number
  score: number
  star_delta: number
}

export interface RankingsResponse {
  repository_id: number
  rankings: RankingHistory[]
}

export interface SearchResponse {
  query: string
  page: number
  per_page: number
  total: number
  items: Repository[]
}
