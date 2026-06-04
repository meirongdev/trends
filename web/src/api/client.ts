import type {
  TrendingResponse,
  LanguageCount,
  RepositoryDetail,
  SnapshotsResponse,
  RankingsResponse,
  SearchResponse,
  TopicCount,
  TopicResponse,
  DevelopersResponse,
  ArchiveResponse,
  Stats,
  Period,
} from './types'

// 同源:生产时前端由 Go 托管;dev 时 Vite 代理 /api 到后端。
const BASE = '/api/v1'

type QueryValue = string | number | undefined

function qs(params: Record<string, QueryValue>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}

async function request<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) {
    let msg = `request failed: ${res.status}`
    try {
      const body = (await res.json()) as { error?: string }
      if (body?.error) msg = body.error
    } catch {
      // ignore non-JSON error bodies
    }
    throw new Error(msg)
  }
  return (await res.json()) as T
}

export function getTrending(p: {
  period?: Period
  language?: string
  date?: string
  page?: number
  perPage?: number
}): Promise<TrendingResponse> {
  return request<TrendingResponse>(
    `${BASE}/trending${qs({ period: p.period, language: p.language, date: p.date, page: p.page, per_page: p.perPage })}`,
  )
}

export function getLanguages(p: { period?: Period; date?: string } = {}): Promise<LanguageCount[]> {
  return request<LanguageCount[]>(`${BASE}/languages${qs({ period: p.period, date: p.date })}`)
}

export function getRepository(id: number | string): Promise<RepositoryDetail> {
  return request<RepositoryDetail>(`${BASE}/repositories/${id}`)
}

export function getRepositorySnapshots(
  id: number | string,
  p: { from?: string; to?: string } = {},
): Promise<SnapshotsResponse> {
  return request<SnapshotsResponse>(
    `${BASE}/repositories/${id}/snapshots${qs({ from: p.from, to: p.to })}`,
  )
}

export function getRepositoryRankings(id: number | string): Promise<RankingsResponse> {
  return request<RankingsResponse>(`${BASE}/repositories/${id}/rankings`)
}

export function search(p: {
  q: string
  language?: string
  page?: number
  perPage?: number
}): Promise<SearchResponse> {
  return request<SearchResponse>(
    `${BASE}/search${qs({ q: p.q, language: p.language, page: p.page, per_page: p.perPage })}`,
  )
}

export function getTopics(): Promise<TopicCount[]> {
  return request<TopicCount[]>(`${BASE}/topics`)
}

export function getTopic(slug: string, page?: number): Promise<TopicResponse> {
  return request<TopicResponse>(`${BASE}/topics/${encodeURIComponent(slug)}${qs({ page })}`)
}

export function getDevelopers(period?: Period, page?: number): Promise<DevelopersResponse> {
  return request<DevelopersResponse>(`${BASE}/developers${qs({ period, page })}`)
}

export function getStats(): Promise<Stats> {
  return request<Stats>(`${BASE}/stats`)
}

export function getArchive(period?: Period, page?: number): Promise<ArchiveResponse> {
  return request<ArchiveResponse>(`${BASE}/archive${qs({ period, page })}`)
}

export async function submitRepository(fullName: string): Promise<{ id: number; status: string }> {
  const res = await fetch(`${BASE}/submissions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ full_name: fullName }),
  })
  if (!res.ok) {
    let msg = `request failed: ${res.status}`
    try {
      const b = (await res.json()) as { error?: string }
      if (b?.error) msg = b.error
    } catch {
      // ignore non-JSON error bodies
    }
    throw new Error(msg)
  }
  return (await res.json()) as { id: number; status: string }
}
