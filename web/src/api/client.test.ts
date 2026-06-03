import { describe, it, expect, vi, afterEach } from 'vitest'
import { getTrending, search, getRepository } from './client'

function mockFetchOnce(body: unknown, ok = true, status = 200) {
  return vi.fn().mockResolvedValue({
    ok,
    status,
    json: async () => body,
  } as Response)
}

describe('api client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('getTrending builds query string and parses response', async () => {
    const fetchMock = mockFetchOnce({
      period: 'weekly', date: '2026-06-10', page: 2, per_page: 25, total: 0, items: [],
    })
    vi.stubGlobal('fetch', fetchMock)

    const res = await getTrending({ period: 'weekly', language: 'Go', page: 2 })
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/trending')
    expect(url).toContain('period=weekly')
    expect(url).toContain('language=Go')
    expect(url).toContain('page=2')
    expect(res.period).toBe('weekly')
  })

  it('omits empty/undefined params', async () => {
    const fetchMock = mockFetchOnce({ items: [] })
    vi.stubGlobal('fetch', fetchMock)
    await getTrending({})
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).not.toContain('language=')
    expect(url).not.toContain('date=')
  })

  it('maps perPage to per_page', async () => {
    const fetchMock = mockFetchOnce({ items: [] })
    vi.stubGlobal('fetch', fetchMock)
    await search({ q: 'react', perPage: 10 })
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('q=react')
    expect(url).toContain('per_page=10')
  })

  it('throws on non-ok response (using error message when present)', async () => {
    const fetchMock = mockFetchOnce({ error: 'repository not found' }, false, 404)
    vi.stubGlobal('fetch', fetchMock)
    await expect(getRepository(999)).rejects.toThrow('repository not found')
  })
})
