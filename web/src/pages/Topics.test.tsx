import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Topics } from './Topics'
import * as client from '../api/client'

describe('Topics page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getTopics').mockResolvedValue([
      { slug: 'ai', name: 'ai', count: 12 },
      { slug: 'cli', name: 'cli', count: 3 },
    ])
  })

  it('lists topics with counts', async () => {
    render(
      <MemoryRouter>
        <Topics />
      </MemoryRouter>,
    )
    expect(await screen.findByText('ai')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('cli')).toBeInTheDocument()
  })
})
