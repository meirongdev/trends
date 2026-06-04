import { useEffect, useState } from 'react'
import { getStats } from '../api/client'
import type { Stats as StatsData } from '../api/types'

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg border border-slate-200 p-4">
      <div className="text-2xl font-bold tabular-nums">{value}</div>
      <div className="text-sm text-slate-500">{label}</div>
    </div>
  )
}

export function Stats() {
  const [data, setData] = useState<StatsData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    getStats()
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
  }, [])

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">站点统计</h1>
      <p className="text-sm text-slate-500">数据采集与排行榜的整体概况。</p>
      {loading && <p className="text-slate-500">加载中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {data && (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <StatCard label="活跃仓库" value={data.active_repos.toLocaleString()} />
            <StatCard label="快照总数" value={data.total_snapshots.toLocaleString()} />
            <StatCard label="语言数" value={data.languages.toLocaleString()} />
            <StatCard label="话题数" value={data.topics.toLocaleString()} />
            <StatCard label="开发者" value={data.developers.toLocaleString()} />
          </div>
          <dl className="space-y-1 text-sm text-slate-600">
            <div className="flex gap-2">
              <dt className="text-slate-400">最新榜单日期</dt>
              <dd>{data.latest_ranking_date || '—'}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="text-slate-400">最近采集时间</dt>
              <dd>{data.last_synced_at || '—'}</dd>
            </div>
          </dl>
        </>
      )}
    </div>
  )
}
