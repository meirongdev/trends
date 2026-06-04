import { useEffect, useState } from 'react'
import { getDevelopers } from '../api/client'
import type { DevelopersResponse } from '../api/types'

export function Developers() {
  const [data, setData] = useState<DevelopersResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    getDevelopers()
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
      <h1 className="text-xl font-bold">开发者排名</h1>
      <p className="text-sm text-slate-500">按其仓库登上日榜的累计次数排名。</p>
      {loading && <p className="text-slate-500">加载中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {data && data.items.length === 0 && !loading && <p className="text-slate-500">暂无数据。</p>}
      <ul>
        {data?.items.map((dev, i) => (
          <li key={dev.login} className="flex items-center gap-3 border-b border-slate-100 py-2">
            <div className="w-6 text-right font-mono text-slate-400">{i + 1}</div>
            {dev.avatar && <img src={dev.avatar} alt="" className="h-8 w-8 rounded-full" />}
            <a
              href={`https://github.com/${dev.login}`}
              target="_blank"
              rel="noreferrer"
              className="flex-1 font-medium text-blue-700 hover:underline"
            >
              {dev.login}
            </a>
            <span className="text-sm text-slate-500">{dev.appearances} 次上榜</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
