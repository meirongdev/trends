import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { TopicDetail } from './TopicDetail'
import * as client from '../api/client'

describe('TopicDetail page', () => {
  beforeEach(() => {
    vi.spyOn(client, 'getTopic').mockResolvedValue({
      slug: 'ai',
      page: 1,
      per_page: 25,
      total: 1,
      items: [
        {
          id: 7,
          full_name: 'a/x',
          owner: 'a',
          name: 'x',
          description: 'd',
          language: 'Go',
          stars: 1000,
          html_url: 'u',
          owner_avatar: 'av',
        },
      ],
    })
  })

  it('shows repos in the topic', async () => {
    render(
      <MemoryRouter initialEntries={['/topics/ai']}>
        <Routes>
          <Route path="/topics/:slug" element={<TopicDetail />} />
        </Routes>
      </MemoryRouter>,
    )
    expect(await screen.findByText('a/x')).toBeInTheDocument()
    expect(client.getTopic as unknown as ReturnType<typeof vi.fn>).toHaveBeenCalledWith('ai')
  })
})
