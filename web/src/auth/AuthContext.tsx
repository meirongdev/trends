import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import type { AuthUser } from '../api/types'
import { getMe, logout as apiLogout } from './client'

interface AuthState {
  user: AuthUser | null
  providers: string[]
  loading: boolean
  refresh: () => void
  logout: () => Promise<void>
}

const Ctx = createContext<AuthState | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [providers, setProviders] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    getMe()
      .then((m) => {
        if (!cancelled) {
          setUser(m.user)
          setProviders(m.providers)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [tick])

  const value: AuthState = {
    user,
    providers,
    loading,
    refresh: () => setTick((t) => t + 1),
    logout: async () => {
      await apiLogout()
      setTick((t) => t + 1)
    },
  }
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth(): AuthState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth must be used within AuthProvider')
  return v
}
