import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { Submit } from './Submit'
import * as client from '../api/client'

function renderSubmit() {
  return render(
    <MemoryRouter>
      <Submit />
    </MemoryRouter>,
  )
}

describe('Submit page', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('submits a repo and shows success', async () => {
    vi.spyOn(client, 'submitRepository').mockResolvedValue({ id: 1, status: 'pending' })
    renderSubmit()
    await userEvent.type(screen.getByLabelText(/owner\/repo/i), 'octocat/hello')
    await userEvent.click(screen.getByRole('button', { name: /提交/ }))
    expect(await screen.findByText(/已提交|收到/)).toBeInTheDocument()
    expect(client.submitRepository as unknown as ReturnType<typeof vi.fn>).toHaveBeenCalledWith('octocat/hello')
  })

  it('shows the error message on failure', async () => {
    vi.spyOn(client, 'submitRepository').mockRejectedValue(new Error('full_name must be owner/repo'))
    renderSubmit()
    await userEvent.type(screen.getByLabelText(/owner\/repo/i), 'bad')
    await userEvent.click(screen.getByRole('button', { name: /提交/ }))
    expect(await screen.findByText(/must be owner\/repo/)).toBeInTheDocument()
  })
})
