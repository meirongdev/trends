import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { Search } from './Search'
import * as client from '../api/client'

function renderSearch(path = '/search') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/search" element={<Search />} />
        <Route path="/repositories/:id" element={<div>detail</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Search page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'search').mockResolvedValue({
      query: 'react',
      page: 1,
      per_page: 25,
      total: 1,
      items: [
        {
          id: 7,
          full_name: 'facebook/react',
          owner: 'facebook',
          name: 'react',
          description: 'A library',
          language: 'JavaScript',
          stars: 5000,
          html_url: 'u',
          owner_avatar: 'av',
        },
      ],
    })
  })

  it('submitting a query shows results and total', async () => {
    renderSearch()
    await userEvent.type(screen.getByLabelText('搜索'), 'react')
    await userEvent.click(screen.getByRole('button', { name: '搜索' }))
    expect(await screen.findByText('facebook/react')).toBeInTheDocument()
    expect(screen.getByText(/共 1 个结果/)).toBeInTheDocument()
  })

  it('runs the query from the URL on load', async () => {
    renderSearch('/search?q=react')
    expect(await screen.findByText('facebook/react')).toBeInTheDocument()
    expect(client.search as unknown as ReturnType<typeof vi.fn>).toHaveBeenCalled()
  })
})
