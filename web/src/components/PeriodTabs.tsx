import type { Period } from '../api/types'

const PERIODS: Period[] = ['daily', 'weekly', 'monthly', 'yearly']
const LABELS: Record<Period, string> = { daily: '日', weekly: '周', monthly: '月', yearly: '年' }

export function PeriodTabs({ active, onChange }: { active: Period; onChange: (p: Period) => void }) {
  return (
    <div className="flex gap-1" role="tablist">
      {PERIODS.map((p) => (
        <button
          key={p}
          role="tab"
          aria-selected={p === active}
          onClick={() => onChange(p)}
          className={`rounded px-3 py-1 text-sm ${
            p === active ? 'bg-slate-900 text-white' : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
          }`}
        >
          {LABELS[p]}
        </button>
      ))}
    </div>
  )
}
