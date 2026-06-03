import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getRepository, getRepositorySnapshots, getRepositoryRankings } from '../api/client'
import type { RepositoryDetail as RepoDetail, Snapshot, RankingHistory } from '../api/types'
import { StarChart } from '../components/StarChart'

export function RepositoryDetail() {
  const { id } = useParams()
  const [repo, setRepo] = useState<RepoDetail | null>(null)
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [rankings, setRankings] = useState<RankingHistory[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return
    let cancelled = false
    setLoading(true)
    setError(null)
    Promise.all([getRepository(id), getRepositorySnapshots(id), getRepositoryRankings(id)])
      .then(([r, s, k]) => {
        if (cancelled) return
        setRepo(r)
        setSnapshots(s.snapshots)
        setRankings(k.rankings)
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
  }, [id])

  if (loading) return <p className="text-slate-500">加载中…</p>
  if (error) return <p className="text-red-600">出错了：{error}</p>
  if (!repo) return <p className="text-slate-500">未找到。</p>

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        {repo.owner_avatar && <img src={repo.owner_avatar} alt="" className="h-12 w-12 rounded" />}
        <div className="min-w-0">
          <a
            href={repo.html_url}
            target="_blank"
            rel="noreferrer"
            className="text-xl font-bold text-blue-700 hover:underline"
          >
            {repo.full_name}
          </a>
          {repo.description && <p className="text-slate-600">{repo.description}</p>}
          <div className="mt-1 flex flex-wrap gap-3 text-sm text-slate-500">
            {repo.language && <span>{repo.language}</span>}
            <span>★ {repo.stars.toLocaleString()}</span>
            <span>Forks {repo.forks.toLocaleString()}</span>
            <span>Issues {repo.open_issues.toLocaleString()}</span>
            {repo.best_daily_rank != null && (
              <span className="text-green-700">最佳日榜 #{repo.best_daily_rank}</span>
            )}
          </div>
        </div>
      </div>

      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700">Star 历史</h2>
        {snapshots.length > 0 ? (
          <StarChart snapshots={snapshots} />
        ) : (
          <p className="text-sm text-slate-400">暂无快照数据。</p>
        )}
      </section>

      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700">上榜历史</h2>
        {rankings.length > 0 ? (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500">
                <th className="py-1">日期</th>
                <th>周期</th>
                <th>名次</th>
                <th>增量</th>
              </tr>
            </thead>
            <tbody>
              {rankings.map((r, i) => (
                <tr key={i} className="border-t border-slate-100">
                  <td className="py-1">{r.date}</td>
                  <td>{r.period}</td>
                  <td>#{r.rank}</td>
                  <td className="text-green-600">+{r.star_delta.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="text-sm text-slate-400">还没上过榜。</p>
        )}
      </section>
    </div>
  )
}
