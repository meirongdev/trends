import { Link, Outlet } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

export function Layout() {
  const { user, logout } = useAuth()
  return (
    <div className="min-h-screen bg-white text-slate-900">
      <header className="border-b border-slate-200">
        <div className="mx-auto flex max-w-4xl items-center gap-4 px-4 py-3">
          <Link to="/" className="text-lg font-bold">
            trends
          </Link>
          <span className="text-sm text-slate-500">GitHub 趋势仓库</span>
          <Link to="/trending/developers" className="ml-auto text-sm text-blue-700 hover:underline">
            开发者
          </Link>
          <Link to="/archive" className="text-sm text-blue-700 hover:underline">
            归档
          </Link>
          <Link to="/stats" className="text-sm text-blue-700 hover:underline">
            统计
          </Link>
          <Link to="/topics" className="text-sm text-blue-700 hover:underline">
            话题
          </Link>
          <Link to="/search" className="text-sm text-blue-700 hover:underline">
            搜索
          </Link>
          <Link to="/submit" className="text-sm text-blue-700 hover:underline">
            提交
          </Link>
          {user ? (
            <span className="flex items-center gap-2 text-sm">
              {user.avatar_url && <img src={user.avatar_url} alt="" className="h-6 w-6 rounded-full" />}
              <span className="text-slate-600">{user.login}</span>
              <button onClick={() => logout()} className="text-blue-700 hover:underline">
                登出
              </button>
            </span>
          ) : null}
        </div>
      </header>
      <main className="mx-auto max-w-4xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
