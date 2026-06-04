import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Archive } from './Archive'
import * as client from '../api/client'

describe('Archive page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getArchive').mockResolvedValue({
      period: 'daily',
      page: 1,
      per_page: 25,
      total: 1,
      items: [
        {
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
          appearances: 5,
          best_rank: 2,
          peak_star_delta: 320,
          first_ranked: '2026-05-01',
          last_ranked: '2026-06-10',
        },
      ],
    })
  })

  it('lists archived repos with appearance counts and best rank', async () => {
    render(
      <MemoryRouter>
        <Archive />
      </MemoryRouter>,
    )
    expect(await screen.findByText('a/x')).toBeInTheDocument()
    expect(screen.getByText(/5 次上榜/)).toBeInTheDocument()
    expect(screen.getByText(/#2/)).toBeInTheDocument()
  })
})
