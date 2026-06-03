import { Link } from 'react-router-dom'
import type { TrendingItem } from '../api/types'

export function RepoRow({ item }: { item: TrendingItem }) {
  const r = item.repository
  return (
    <li className="flex items-start gap-3 border-b border-slate-100 py-3">
      <div className="w-8 text-right font-mono text-slate-400">{item.rank}</div>
      {r.owner_avatar && <img src={r.owner_avatar} alt="" className="h-8 w-8 rounded" />}
      <div className="min-w-0 flex-1">
        <Link to={`/repositories/${r.id}`} className="font-medium text-blue-700 hover:underline">
          {r.full_name}
        </Link>
        {r.description && <p className="truncate text-sm text-slate-600">{r.description}</p>}
        <div className="mt-0.5 flex gap-3 text-xs text-slate-500">
          {r.language && <span>{r.language}</span>}
          <span>★ {r.stars.toLocaleString()}</span>
          <span className="text-green-600">+{item.star_delta.toLocaleString()}</span>
        </div>
      </div>
    </li>
  )
}
