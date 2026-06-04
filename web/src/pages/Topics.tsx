import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getTopics } from '../api/client'
import type { TopicCount } from '../api/types'

export function Topics() {
  const [topics, setTopics] = useState<TopicCount[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    getTopics()
      .then((t) => {
        if (!cancelled) setTopics(t)
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
      <h1 className="text-xl font-bold">话题</h1>
      {loading && <p className="text-slate-500">加载中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {!loading && topics.length === 0 && <p className="text-slate-500">暂无话题。</p>}
      <div className="flex flex-wrap gap-2">
        {topics.map((t) => (
          <Link
            key={t.slug}
            to={`/topics/${t.slug}`}
            className="rounded bg-slate-100 px-3 py-1 text-sm hover:bg-slate-200"
          >
            {t.name} <span className="text-slate-400">{t.count}</span>
          </Link>
        ))}
      </div>
    </div>
  )
}
