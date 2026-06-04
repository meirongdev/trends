import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getArchive } from '../api/client'
import type { ArchiveResponse, Period } from '../api/types'
import { PeriodTabs } from '../components/PeriodTabs'

export function Archive() {
  const [period, setPeriod] = useState<Period>('daily')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<ArchiveResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    getArchive(period, page)
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
  }, [period, page])

  const changePeriod = (p: Period) => {
    setPeriod(p)
    setPage(1)
  }

  const pages = data ? Math.max(1, Math.ceil(data.total / data.per_page)) : 1

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">历史归档</h1>
        <PeriodTabs active={period} onChange={changePeriod} />
      </div>
      <p className="text-sm text-slate-500">所有曾经登上榜单的仓库,按累计上榜次数排名。</p>
      {loading && <p className="text-slate-500">加载中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {data && data.items.length === 0 && !loading && <p className="text-slate-500">暂无数据。</p>}
      <ul>
        {data?.items.map((e) => (
          <li key={e.repository.id} className="flex items-center gap-3 border-b border-slate-100 py-2">
            {e.repository.owner_avatar && (
              <img src={e.repository.owner_avatar} alt="" className="h-8 w-8 rounded-full" />
            )}
            <div className="min-w-0 flex-1">
              <Link
                to={`/repositories/${e.repository.id}`}
                className="font-medium text-blue-700 hover:underline"
              >
                {e.repository.full_name}
              </Link>
              <div className="text-xs text-slate-500">
                {e.first_ranked} – {e.last_ranked}
                {e.repository.language ? ` · ${e.repository.language}` : ''}
              </div>
            </div>
            <div className="text-right text-sm">
              <div className="text-slate-700">{e.appearances} 次上榜</div>
              <div className="text-xs text-slate-400">最佳 #{e.best_rank} · 峰值 +{e.peak_star_delta}</div>
            </div>
          </li>
        ))}
      </ul>
      {pages > 1 && (
        <div className="flex items-center gap-2 text-sm">
          <button
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
            className="rounded bg-slate-100 px-2 py-1 disabled:opacity-50"
          >
            上一页
          </button>
          <span className="text-slate-500">
            {page} / {pages}
          </span>
          <button
            disabled={page >= pages}
            onClick={() => setPage(page + 1)}
            className="rounded bg-slate-100 px-2 py-1 disabled:opacity-50"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}
