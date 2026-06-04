import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Stats } from './Stats'
import * as client from '../api/client'

describe('Stats page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getStats').mockResolvedValue({
      active_repos: 1234,
      total_snapshots: 5678,
      languages: 42,
      topics: 99,
      developers: 321,
      latest_ranking_date: '2026-06-10',
      last_synced_at: '2026-06-10T08:00:00Z',
    })
  })

  it('renders aggregate counts', async () => {
    render(
      <MemoryRouter>
        <Stats />
      </MemoryRouter>,
    )
    expect(await screen.findByText('1,234')).toBeInTheDocument()
    expect(screen.getByText('5,678')).toBeInTheDocument()
    expect(screen.getByText('活跃仓库')).toBeInTheDocument()
    expect(screen.getByText('2026-06-10')).toBeInTheDocument()
  })
})
