import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthProvider, useAuth } from './AuthContext'
import * as client from './client'

function Probe() {
  const { user, loading } = useAuth()
  if (loading) return <span>loading</span>
  return <span>{user ? `hi ${user.login}` : 'anon'}</span>
}

describe('AuthContext', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getMe').mockResolvedValue({
      user: { login: 'octocat', avatar_url: 'av', provider: 'github' },
      providers: ['github', 'google'],
    })
  })

  it('exposes the current user after loading', async () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    )
    expect(await screen.findByText('hi octocat')).toBeInTheDocument()
  })
})
