import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

describe('App', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('loads repositories and creates a new repository', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({ repositories: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: 1, owner: 'alice', name: 'castle', path: '/repos/alice/castle.git', created_at: new Date().toISOString() }), { status: 201 }))

    render(<App />)
    expect(await screen.findByText('No repositories yet. Build your first stronghold above.')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Owner'), { target: { value: 'alice' } })
    fireEvent.change(screen.getByLabelText('Repository name'), { target: { value: 'castle' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create repository' }))

    expect(await screen.findByText('castle')).toBeInTheDocument()
    expect(fetchMock).toHaveBeenLastCalledWith('/api/v1/repositories', expect.objectContaining({ method: 'POST' }))
  })

  it('shows API errors', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(JSON.stringify({ error: 'database unavailable' }), { status: 500 }))

    render(<App />)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('database unavailable'))
  })
})
