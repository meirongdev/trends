import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AuthProvider } from '../auth/AuthContext'
import { Submit } from './Submit'
import * as authClient from '../auth/client'

function renderSubmit() {
  return render(
    <AuthProvider>
      <MemoryRouter>
        <Submit />
      </MemoryRouter>
    </AuthProvider>,
  )
}

describe('Submit page', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('shows login buttons when logged out', async () => {
    vi.spyOn(authClient, 'getMe').mockResolvedValue({ user: null, providers: ['github', 'google'] })
    renderSubmit()
    expect(await screen.findByRole('button', { name: /GitHub/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Google/i })).toBeInTheDocument()
  })

  it('shows the submit form when logged in', async () => {
    vi.spyOn(authClient, 'getMe').mockResolvedValue({
      user: { login: 'octocat', avatar_url: 'av', provider: 'github' },
      providers: ['github'],
    })
    renderSubmit()
    expect(await screen.findByPlaceholderText('owner/repo')).toBeInTheDocument()
  })
})
