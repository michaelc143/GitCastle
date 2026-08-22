import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

// The app now starts with a session check (GET /api/v1/me) before rendering.
function mockSession(user: object | null) {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    new Response(JSON.stringify(user), { status: user ? 200 : 401 }),
  )
}

function mockRepositories(repositories: unknown[]) {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    new Response(JSON.stringify({ repositories }), { status: 200 }),
  )
}

describe('App', () => {
  beforeEach(() => {
    window.location.hash = ''
    vi.restoreAllMocks()
  })

  it('loads repositories and links anonymous visitors to sign in', async () => {
    mockSession(null)
    mockRepositories([])

    render(<App />)
    expect(await screen.findByText(/No repositories yet/)).toBeInTheDocument()

    // Anonymous visitors are invited to sign in from the top bar.
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    await waitFor(() => expect(window.location.hash).toBe('#/login'))
  })

  it('shows API errors on the repository list', async () => {
    mockSession(null)
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'database unavailable' }), { status: 500 }),
    )

    render(<App />)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('database unavailable'))
  })

  it('greets a signed-in user and offers sign out', async () => {
    mockSession({ id: 1, username: 'alice', created_at: new Date().toISOString() })
    mockRepositories([
      { id: 1, owner: 'alice', name: 'castle', path: '/repos/alice/castle.git', created_at: new Date().toISOString() },
    ])

    render(<App />)
    expect(await screen.findByText(/Welcome back, alice/)).toBeInTheDocument()
    expect(await screen.findByText('castle')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  })
})
