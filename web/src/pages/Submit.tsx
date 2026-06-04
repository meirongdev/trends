import { useState, type FormEvent } from 'react'
import { submitRepository } from '../api/client'

export function Submit() {
  const [input, setInput] = useState('')
  const [status, setStatus] = useState<'idle' | 'sending' | 'done' | 'error'>('idle')
  const [message, setMessage] = useState('')

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    const fullName = input.trim()
    if (!fullName) return
    setStatus('sending')
    setMessage('')
    try {
      await submitRepository(fullName)
      setStatus('done')
      setMessage(`已提交 ${fullName},等待收录审核。`)
      setInput('')
    } catch (err) {
      setStatus('error')
      setMessage(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="max-w-lg space-y-4">
      <div>
        <h1 className="text-xl font-bold">提交收录</h1>
        <p className="mt-1 text-sm text-slate-500">
          提交一个 GitHub 仓库(<code>owner/repo</code>),通过校验后会纳入趋势追踪。
        </p>
      </div>
      <form onSubmit={onSubmit} className="flex gap-2">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="owner/repo"
          aria-label="owner/repo"
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <button
          type="submit"
          disabled={status === 'sending'}
          className="rounded bg-slate-900 px-3 py-1.5 text-sm text-white disabled:opacity-50"
        >
          {status === 'sending' ? '提交中…' : '提交'}
        </button>
      </form>
      {message && (
        <p className={status === 'error' ? 'text-sm text-red-600' : 'text-sm text-green-700'}>{message}</p>
      )}
    </div>
  )
}
