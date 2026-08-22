import { FormEvent, useEffect, useState } from 'react'
import { createRepository, listRepositories, Repository, User } from './api'
import { navigate } from './router'

type Props = {
  user: User | null
  onRequireLogin: () => void
}

export default function HomePage({ user, onRequireLogin }: Props) {
  const [repositories, setRepositories] = useState<Repository[]>([])
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listRepositories()
      .then(setRepositories)
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Unable to load repositories'))
      .finally(() => setLoading(false))
  }, [])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!user) {
      onRequireLogin()
      return
    }
    setSubmitting(true)
    setError('')
    try {
      const repository = await createRepository(user.username, name.trim())
      navigate(`/${repository.owner}/${repository.name}`)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to create repository')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="shell shell-dashboard">
      <header className="page-intro home-intro">
        <div>
          <p className="eyebrow">Workspace</p>
          <h1>Repositories</h1>
          <p className="page-subtitle">
            {user ? `Welcome back, ${user.username}.` : 'Browse public source and keep your code close.'}
          </p>
        </div>
        <span className="context-chip">Self-hosted</span>
      </header>

      {user && (
        <section className="card create-panel" aria-label="Create a repository">
          <div className="panel-heading">
            <div>
              <p className="section-kicker">Start something new</p>
              <h2>New repository</h2>
            </div>
            <span className="panel-index">01</span>
          </div>
          <form className="create-form" onSubmit={handleSubmit}>
            <label>
              Owner
              <input value={user.username} readOnly aria-label="Owner" />
            </label>
            <label>
              Repository name
              <input
                aria-label="Repository name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="castle"
                required
              />
            </label>
            <button type="submit" disabled={submitting || !name.trim()}>
              {submitting ? 'Creating…' : 'Create repository'}
            </button>
          </form>
        </section>
      )}

      <section className="card" aria-label="Repositories">
        <div className="section-heading">
          <div>
            <p className="section-kicker">Your codebase</p>
            <h2>All repositories</h2>
          </div>
          <span className="count-badge"><span className="count-number">{repositories.length}</span> total</span>
        </div>

        {error && <p className="error" role="alert">{error}</p>}
        {!user && !error && (
          <p className="empty-state">
            Sign in to create repositories and push code. Browsing stays open.
          </p>
        )}
        {loading ? (
          <p className="empty-state">Loading…</p>
        ) : repositories.length === 0 ? (
          <p className="empty-state">No repositories yet. Build your first stronghold above.</p>
        ) : (
          <div className="repository-list">
            {repositories.map((repository) => (
              <a
                className="repository-card"
                key={repository.id}
                href={`#/${repository.owner}/${repository.name}`}
              >
                <div className="repo-icon" aria-hidden="true">R</div>
                <div>
                  <p className="repo-owner">{repository.owner}</p>
                  <h3>{repository.name}</h3>
                </div>
                <span className="row-arrow" aria-hidden="true">↗</span>
              </a>
            ))}
          </div>
        )}
      </section>
    </main>
  )
}
