import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getTopic } from '../api/client'
import type { TopicResponse } from '../api/types'

export function TopicDetail() {
  const { slug } = useParams()
  const [data, setData] = useState<TopicResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!slug) return
    let cancelled = false
    setLoading(true)
    setError(null)
    getTopic(slug)
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
  }, [slug])

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">
        话题：<span className="text-blue-700">#{slug}</span>
      </h1>
      {loading && <p className="text-slate-500">加载中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {data && (
        <>
          <p className="text-sm text-slate-500">共 {data.total} 个仓库</p>
          {data.items.length === 0 ? (
            <p className="text-slate-500">该话题下暂无仓库。</p>
          ) : (
            <ul>
              {data.items.map((r) => (
                <li key={r.id} className="border-b border-slate-100 py-2">
                  <Link
                    to={`/repositories/${r.id}`}
                    className="font-medium text-blue-700 hover:underline"
                  >
                    {r.full_name}
                  </Link>
                  {r.description && <p className="truncate text-sm text-slate-600">{r.description}</p>}
                  <div className="text-xs text-slate-500">
                    {r.language && <span className="mr-3">{r.language}</span>}★{' '}
                    {r.stars.toLocaleString()}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  )
}
