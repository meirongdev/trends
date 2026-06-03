import type { LanguageCount } from '../api/types'

export function LanguageFilter({
  languages,
  active,
  onChange,
}: {
  languages: LanguageCount[]
  active: string
  onChange: (lang: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-1">
      <button
        onClick={() => onChange('')}
        className={`rounded px-2 py-0.5 text-xs ${active === '' ? 'bg-slate-900 text-white' : 'bg-slate-100 hover:bg-slate-200'}`}
      >
        全部
      </button>
      {languages.map((l) => (
        <button
          key={l.language}
          onClick={() => onChange(l.language)}
          className={`rounded px-2 py-0.5 text-xs ${active === l.language ? 'bg-slate-900 text-white' : 'bg-slate-100 hover:bg-slate-200'}`}
        >
          {l.language} <span className="text-slate-400">{l.count}</span>
        </button>
      ))}
    </div>
  )
}
