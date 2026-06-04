import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Developers } from './Developers'
import * as client from '../api/client'

describe('Developers page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getDevelopers').mockResolvedValue({
      period: 'daily',
      page: 1,
      per_page: 25,
      total: 2,
      items: [
        { login: 'alice', avatar: 'av-a', appearances: 5 },
        { login: 'bob', avatar: 'av-b', appearances: 2 },
      ],
    })
  })

  it('lists developers with appearance counts', async () => {
    render(
      <MemoryRouter>
        <Developers />
      </MemoryRouter>,
    )
    expect(await screen.findByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText(/5/)).toBeInTheDocument()
  })
})
