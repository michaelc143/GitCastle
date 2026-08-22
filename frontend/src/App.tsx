import { useEffect, useState } from 'react'
import { logout, me, User } from './api'
import HomePage from './HomePage'
import LoginPage from './LoginPage'
import { CodeBrowser, CommitDetail, CommitHistory } from './RepoPage'
import { navigate, useRoute } from './router'
import './styles.css'

export default function App() {
  const route = useRoute()
  const [user, setUser] = useState<User | null>(null)
  const [sessionChecked, setSessionChecked] = useState(false)

  useEffect(() => {
    me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setSessionChecked(true))
  }, [])

  async function handleLogout() {
    await logout().catch(() => undefined)
    setUser(null)
    navigate('/')
  }

  if (!sessionChecked) {
    return <main className="shell"><p className="empty-state">Loading…</p></main>
  }

  return (
    <>
      <nav className="topbar">
        <a className="brand" href="#/">Git<span>Castle</span></a>
        <div>
          {user ? (
            <>
              <span className="muted">{user.username}</span>{' '}
              <button type="button" className="link" onClick={handleLogout}>Sign out</button>
            </>
          ) : (
            <button type="button" className="link" onClick={() => navigate('/login')}>Sign in</button>
          )}
        </div>
      </nav>

      {route.page === 'home' && <HomePage user={user} onRequireLogin={() => navigate('/login')} />}
      {route.page === 'login' && (
        user
          ? <HomePage user={user} onRequireLogin={() => navigate('/login')} />
          : <LoginPage onAuthenticated={(username) => {
              setUser({ id: 0, username, created_at: new Date().toISOString() })
              navigate('/')
            }} onGoHome={() => navigate('/')} />
      )}
      {route.page === 'repo' && route.tab === 'code' && (
        <RepoShell title={`${route.owner}/${route.name}`}>
          <CodeBrowser owner={route.owner} name={route.name} rev={route.rev} filePath={route.filePath} />
        </RepoShell>
      )}
      {route.page === 'repo' && route.tab === 'commits' && !route.commitHash && (
        <RepoShell title={`${route.owner}/${route.name}`}>
          <CommitHistory owner={route.owner} name={route.name} rev={route.rev} />
        </RepoShell>
      )}
      {route.page === 'repo' && route.commitHash && (
        <RepoShell title={`${route.owner}/${route.name}`}>
          <CommitDetail owner={route.owner} name={route.name} hash={route.commitHash} />
        </RepoShell>
      )}
    </>
  )
}

function RepoShell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <main className="shell">
      <header className="repo-header">
        <h1>{title}</h1>
      </header>
      {children}
    </main>
  )
}
