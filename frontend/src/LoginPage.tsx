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
    <main className="shell shell-narrow">
      <header className="hero">
        <div>
          <p className="eyebrow">A self-hosted forge</p>
          <h1>Git<span>Castle</span></h1>
        </div>
        <div className="castle-mark" aria-hidden="true">♜</div>
      </header>

      <section className="card" aria-label={mode === 'login' ? 'Sign in' : 'Create account'}>
        <h2>{mode === 'login' ? 'Sign in' : 'Create your account'}</h2>
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
        <p className="muted">
          {mode === 'login' ? (
            <>New here?{' '}
              <button type="button" className="link" onClick={() => { setMode('register'); setError('') }}>
                Create an account
              </button>
            </>
          ) : (
            <>Already have an account?{' '}
              <button type="button" className="link" onClick={() => { setMode('login'); setError('') }}>
                Sign in
              </button>
            </>
          )}
          {' · '}
          <button type="button" className="link" onClick={onGoHome}>Browse without signing in</button>
        </p>
      </section>
    </main>
  )
}
