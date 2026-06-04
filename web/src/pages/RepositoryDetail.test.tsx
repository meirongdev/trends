import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { RepositoryDetail } from './RepositoryDetail'
import * as client from '../api/client'

describe('RepositoryDetail page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getRepository').mockResolvedValue({
      id: 7,
      full_name: 'a/x',
      owner: 'a',
      name: 'x',
      description: 'a demo repo',
      language: 'Go',
      homepage: '',
      html_url: 'https://github.com/a/x',
      owner_avatar: 'av',
      stars: 1000,
      forks: 50,
      open_issues: 5,
      watchers: 9,
      repo_created_at: '2024-01-01T00:00:00Z',
      best_daily_rank: 3,
      topics: ['ai', 'cli'],
    })
    vi.spyOn(client, 'getRepositorySnapshots').mockResolvedValue({
      repository_id: 7,
      snapshots: [
        { date: '2026-06-09', stars: 980, forks: 49, open_issues: 5, watchers: 9, star_delta: 30 },
        { date: '2026-06-10', stars: 1000, forks: 50, open_issues: 5, watchers: 9, star_delta: 20 },
      ],
    })
    vi.spyOn(client, 'getRepositoryRankings').mockResolvedValue({
      repository_id: 7,
      rankings: [{ period: 'daily', date: '2026-06-10', rank: 3, score: 0.7, star_delta: 20 }],
    })
  })

  it('renders repo metadata, star chart, and ranking history', async () => {
    render(
      <MemoryRouter initialEntries={['/repositories/7']}>
        <Routes>
          <Route path="/repositories/:id" element={<RepositoryDetail />} />
        </Routes>
      </MemoryRouter>,
    )
    expect(await screen.findByText('a/x')).toBeInTheDocument()
    expect(screen.getByText(/最佳日榜 #3/)).toBeInTheDocument()
    expect(screen.getByText(/Forks 50/)).toBeInTheDocument()
    expect(screen.getByTestId('star-chart')).toBeInTheDocument()
    expect(screen.getByText('#3')).toBeInTheDocument() // ranking history row
  })
})
