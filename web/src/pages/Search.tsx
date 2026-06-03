import { useEffect, useState, type FormEvent } from 'react'
import { useSearchParams, Link } from 'react-router-dom'
import { search } from '../api/client'
import type { SearchResponse } from '../api/types'

export function Search() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') ?? ''
  const [input, setInput] = useState(q)
  const [data, setData] = useState<SearchResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!q) {
      setData(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    search({ q })
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
  }, [q])

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    const next = new URLSearchParams()
    if (input.trim()) next.set('q', input.trim())
    setSearchParams(next)
  }

  return (
    <div className="space-y-4">
      <form onSubmit={onSubmit} className="flex gap-2">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="搜索仓库…"
          aria-label="搜索"
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <button type="submit" className="rounded bg-slate-900 px-3 py-1.5 text-sm text-white">
          搜索
        </button>
      </form>
      {loading && <p className="text-slate-500">搜索中…</p>}
      {error && <p className="text-red-600">出错了：{error}</p>}
      {data && (
        <>
          <p className="text-sm text-slate-500">共 {data.total} 个结果</p>
          {data.items.length === 0 ? (
            <p className="text-slate-500">没有匹配的仓库。</p>
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
