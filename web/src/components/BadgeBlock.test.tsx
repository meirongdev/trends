import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BadgeBlock } from './BadgeBlock'

describe('BadgeBlock', () => {
  it('shows the badge image and the markdown embed snippet', () => {
    render(<BadgeBlock id={7} fullName="a/x" />)
    const img = screen.getByAltText(/trends badge/i) as HTMLImageElement
    expect(img.getAttribute('src')).toBe('/api/v1/repositories/7/badge.svg')
    expect(
      screen.getByText(/!\[.*\]\(\/api\/v1\/repositories\/7\/badge\.svg\)/),
    ).toBeInTheDocument()
  })
})
