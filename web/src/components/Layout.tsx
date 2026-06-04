import { Link, Outlet } from 'react-router-dom'

export function Layout() {
  return (
    <div className="min-h-screen bg-white text-slate-900">
      <header className="border-b border-slate-200">
        <div className="mx-auto flex max-w-4xl items-center gap-4 px-4 py-3">
          <Link to="/" className="text-lg font-bold">
            trends
          </Link>
          <span className="text-sm text-slate-500">GitHub 趋势仓库</span>
          <Link to="/search" className="ml-auto text-sm text-blue-700 hover:underline">
            搜索
          </Link>
          <Link to="/submit" className="text-sm text-blue-700 hover:underline">
            提交
          </Link>
        </div>
      </header>
      <main className="mx-auto max-w-4xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
