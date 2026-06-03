import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { Trending } from './Trending'
import * as client from '../api/client'

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/" element={<Trending />} />
        <Route path="/trending/:period" element={<Trending />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Trending page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getLanguages').mockResolvedValue([{ language: 'Go', count: 2 }])
    vi.spyOn(client, 'getTrending').mockResolvedValue({
      period: 'daily',
      date: '2026-06-10',
      page: 1,
      per_page: 25,
      total: 1,
      items: [
        {
          rank: 1,
          score: 0.9,
          star_delta: 120,
          repository: {
            id: 7,
            full_name: 'a/x',
            owner: 'a',
            name: 'x',
            description: 'desc',
            language: 'Go',
            stars: 1000,
            html_url: 'u',
            owner_avatar: 'av',
          },
        },
      ],
    })
  })

  it('renders trending items from the API', async () => {
    renderAt('/')
    expect(await screen.findByText('a/x')).toBeInTheDocument()
    expect(screen.getByText(/\+120/)).toBeInTheDocument()
  })

  it('switching period refetches with the new period', async () => {
    const spy = client.getTrending as unknown as ReturnType<typeof vi.fn>
    renderAt('/')
    await screen.findByText('a/x')
    await userEvent.click(screen.getByRole('tab', { name: '周' }))
    await waitFor(() => {
      expect(
        spy.mock.calls.some((c) => (c[0] as { period?: string }).period === 'weekly'),
      ).toBe(true)
    })
  })
})
