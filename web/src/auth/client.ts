import type { MeResponse } from '../api/types'

const BASE = '/api/v1/auth'

export async function getMe(): Promise<MeResponse> {
  const res = await fetch(`${BASE}/me`, { credentials: 'same-origin' })
  if (!res.ok) return { user: null, providers: [] }
  return (await res.json()) as MeResponse
}

export function login(provider: string): void {
  window.location.href = `${BASE}/login?provider=${encodeURIComponent(provider)}`
}

export async function logout(): Promise<void> {
  await fetch(`${BASE}/logout`, { method: 'POST', credentials: 'same-origin' })
}
