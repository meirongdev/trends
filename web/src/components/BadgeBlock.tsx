import { useState } from 'react'

export function BadgeBlock({ id, fullName }: { id: number; fullName: string }) {
  const badgeURL = `/api/v1/repositories/${id}/badge.svg`
  const markdown = `[![${fullName} on trends](${badgeURL})](/repositories/${id})`
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(markdown)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // 剪贴板不可用时忽略
    }
  }

  return (
    <section>
      <h2 className="mb-2 text-sm font-semibold text-slate-700">徽章</h2>
      <div className="flex items-center gap-3">
        <img src={badgeURL} alt={`${fullName} trends badge`} height={20} />
        <button
          onClick={copy}
          className="rounded bg-slate-100 px-2 py-1 text-xs hover:bg-slate-200"
        >
          {copied ? '已复制' : '复制 Markdown'}
        </button>
      </div>
      <code className="mt-2 block overflow-x-auto rounded bg-slate-50 p-2 text-xs text-slate-700">
        {markdown}
      </code>
    </section>
  )
}
