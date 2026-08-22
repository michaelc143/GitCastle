import { FormEvent, useEffect, useState } from 'react'
import { createRepository, listRepositories, Repository } from './api'
import './styles.css'

export default function App() {
  const [repositories, setRepositories] = useState<Repository[]>([])
  const [owner, setOwner] = useState('')
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listRepositories()
      .then(setRepositories)
      .catch((reason: Error) => setError(reason.message))
      .finally(() => setLoading(false))
  }, [])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      const repository = await createRepository(owner.trim(), name.trim())
      setRepositories((current) => [repository, ...current])
      setOwner('')
      setName('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to create repository')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="shell">
      <header className="hero">
        <div>
          <p className="eyebrow">A self-hosted forge</p>
          <h1>Git<span>Castle</span></h1>
          <p className="subtitle">Your code. Your rules. A sturdy home for every repository.</p>
        </div>
        <div className="castle-mark" aria-hidden="true">♜</div>
      </header>

      <section className="workspace" aria-label="Repository workspace">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Your workspace</p>
            <h2>Repositories</h2>
          </div>
          <span className="count-badge">{repositories.length}</span>
        </div>

        <form className="create-form" onSubmit={handleSubmit}>
          <label>
            Owner
            <input aria-label="Owner" value={owner} onChange={(event) => setOwner(event.target.value)} placeholder="alice" required />
          </label>
          <label>
            Repository name
            <input aria-label="Repository name" value={name} onChange={(event) => setName(event.target.value)} placeholder="castle" required />
          </label>
          <button type="submit" disabled={submitting}>
            {submitting ? 'Creating…' : 'Create repository'}
          </button>
        </form>

        {error && <p className="error" role="alert">{error}</p>}

        {loading ? (
          <p className="empty-state">Loading your repositories…</p>
        ) : repositories.length === 0 ? (
          <p className="empty-state">No repositories yet. Build your first stronghold above.</p>
        ) : (
          <div className="repository-list">
            {repositories.map((repository) => (
              <article className="repository-card" key={repository.id}>
                <div className="repo-icon">⌘</div>
                <div>
                  <p className="repo-owner">{repository.owner}</p>
                  <h3>{repository.name}</h3>
                </div>
                <code>{repository.path}</code>
              </article>
            ))}
          </div>
        )}
      </section>
    </main>
  )
}
