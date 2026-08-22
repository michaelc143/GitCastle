import { FormEvent, useState } from 'react'
import { login, register } from './api'

type Props = {
  onAuthenticated: (username: string) => void
  onGoHome: () => void
}

export default function LoginPage({ onAuthenticated, onGoHome }: Props) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      if (mode === 'login') {
        const user = await login(username.trim(), password)
        onAuthenticated(user.username)
      } else {
        await register(username.trim(), password)
        await login(username.trim(), password)
        onAuthenticated(username.trim())
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Something went wrong')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="shell shell-narrow auth-shell">
      <header className="auth-intro page-intro">
        <div>
          <p className="eyebrow">Account</p>
          <h1>{mode === 'login' ? 'Welcome back' : 'Create an account'}</h1>
          <p className="page-subtitle">Access your repositories and collaborate from your own forge.</p>
        </div>
      </header>

      <section className="card auth-card" aria-label={mode === 'login' ? 'Sign in' : 'Create account'}>
        <div className="auth-mode" aria-label="Account action">
          <button
            type="button"
            className={`mode-option${mode === 'login' ? ' active' : ''}`}
            aria-pressed={mode === 'login'}
            onClick={() => { setMode('login'); setError('') }}
          >
            Sign in
          </button>
          <button
            type="button"
            className={`mode-option${mode === 'register' ? ' active' : ''}`}
            aria-pressed={mode === 'register'}
            onClick={() => { setMode('register'); setError('') }}
          >
            Create account
          </button>
        </div>
        <form onSubmit={handleSubmit} className="stack">
          <label>
            Username
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="alice"
              autoComplete="username"
              required
            />
          </label>
          <label>
            Password
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder={mode === 'register' ? 'at least 8 characters' : 'password'}
              autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
              required
            />
          </label>
          {error && <p className="error" role="alert">{error}</p>}
          <button type="submit" disabled={busy}>
            {busy ? 'Working…' : mode === 'login' ? 'Sign in' : 'Create account & sign in'}
          </button>
        </form>
        <p className="auth-footer muted">
          <button type="button" className="link" onClick={onGoHome}>Browse without signing in</button>
        </p>
      </section>
    </main>
  )
}
