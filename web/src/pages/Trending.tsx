import { useEffect, useState, useCallback } from 'react'
import { useParams, useSearchParams, useNavigate } from 'react-router-dom'
import { getTrending, getLanguages } from '../api/client'
import type { Period, TrendingResponse, LanguageCount } from '../api/types'
import { PeriodTabs } from '../components/PeriodTabs'
import { LanguageFilter } from '../components/LanguageFilter'
import { RepoRow } from '../components/RepoRow'

const VALID: Period[] = ['daily', 'weekly', 'monthly']

export function Trending() {
  const { period: periodParam } = useParams()
  const period: Period = VALID.includes(periodParam as Period) ? (periodParam as Period) : 'daily'
  const [searchParams, setSearchParams] = useSearchParams()
  const language = searchParams.get('language') ?? ''
  const page = Number(searchParams.get('page') ?? '1') || 1
  const navigate = useNavigate()

  const [data, setData] = useState<TrendingResponse | null>(null)
  const [languages, setLanguages] = useState<LanguageCount[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    getTrending({ period, language: language || undefined, page })
      .then((d) => {
        if (!cancelled) setData(d)
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [period, language, page])

  useEffect(() => {
    let cancelled = false
    getLanguages({ period })
      .then((l) => {
        if (!cancelled) setLanguages(l)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [period])

  const setPeriod = useCallback(
    (p: Period) => {
      navigate(`/trending/${p}${language ? `?language=${encodeURIComponent(language)}` : ''}`)
    },
    [navigate, language],
  )

  const setLanguage = useCallback(
    (lang: string) => {
      const next = new URLSearchParams(searchParams)
      if (lang) next.set('language', lang)
      else next.delete('language')
      next.delete('page')
      setSearchParams(next)
    },
    [searchParams, setSearchParams],
  )

  const goPage = useCallback(
    (p: number) => {
      const next = new URLSearchParams(searchParams)
      next.set('page', String(p))
      setSearchParams(next)
    },
    [searchParams, setSearchParams],
  )

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">趋势榜</h1>
        <PeriodTabs active={period} onChange={setPeriod} />
      </div>
      <LanguageFilter languages={languages} active={language} onChange={setLanguage} />
      {loading && <p className="text-slate-500">加载中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {data && (
        <>
          <ul>
            {data.items.map((it) => (
              <RepoRow key={it.repository.id} item={it} />
            ))}
          </ul>
          {data.items.length === 0 && !loading && <p className="text-slate-500">暂无数据。</p>}
          <Pagination page={data.page} perPage={data.per_page} total={data.total} onPage={goPage} />
        </>
      )}
    </div>
  )
}

function Pagination({
  page,
  perPage,
  total,
  onPage,
}: {
  page: number
  perPage: number
  total: number
  onPage: (p: number) => void
}) {
  const pages = Math.max(1, Math.ceil(total / perPage))
  if (pages <= 1) return null
  return (
    <div className="flex items-center gap-2 text-sm">
      <button
        disabled={page <= 1}
        onClick={() => onPage(page - 1)}
        className="rounded bg-slate-100 px-2 py-1 disabled:opacity-50"
      >
        上一页
      </button>
      <span className="text-slate-500">
        {page} / {pages}
      </span>
      <button
        disabled={page >= pages}
        onClick={() => onPage(page + 1)}
        className="rounded bg-slate-100 px-2 py-1 disabled:opacity-50"
      >
        下一页
      </button>
    </div>
  )
}
