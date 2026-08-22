import { useEffect, useState } from 'react'
import { logout, me, User } from './api'
import HomePage from './HomePage'
import LoginPage from './LoginPage'
import { IssuesList, IssueDetail } from './IssuesPage'
import { PullRequestsList, PullRequestDetail } from './PullsPage'
import { CodeBrowser, CommitDetail, CommitHistory } from './RepoPage'
import { navigate, Route, useRoute } from './router'
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
        <div className="topbar-inner">
          <a className="brand" href="#/"><span className="brand-mark">G</span><span className="brand-name">GitCastle</span></a>
          <div className="topbar-user">
          {user ? (
            <>
              <span className="muted">{user.username}</span>{' '}
              <button type="button" className="link" onClick={handleLogout}>Sign out</button>
            </>
          ) : (
            <button type="button" className="link" onClick={() => navigate('/login')}>Sign in</button>
          )}
          </div>
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
      {route.page === 'repo' && (
        <RepoShell title={`${route.owner}/${route.name}`} route={route}>
          {route.tab === 'code' && (
            <CodeBrowser owner={route.owner} name={route.name} rev={route.rev} filePath={route.filePath} />
          )}
          {route.tab === 'commits' && !route.commitHash && (
            <CommitHistory owner={route.owner} name={route.name} rev={route.rev} />
          )}
          {route.commitHash && (
            <CommitDetail owner={route.owner} name={route.name} hash={route.commitHash} />
          )}
          {route.tab === 'issues' && route.subjectNumber === undefined && (
            <IssuesList owner={route.owner} name={route.name} signedIn={user !== null} />
          )}
          {route.tab === 'issues' && route.subjectNumber !== undefined && (
            <IssueDetail owner={route.owner} name={route.name} number={route.subjectNumber} signedIn={user !== null} />
          )}
          {route.tab === 'pulls' && route.subjectNumber === undefined && (
            <PullRequestsList owner={route.owner} name={route.name} signedIn={user !== null} />
          )}
          {route.tab === 'pulls' && route.subjectNumber !== undefined && (
            <PullRequestDetail owner={route.owner} name={route.name} number={route.subjectNumber} signedIn={user !== null} />
          )}
        </RepoShell>
      )}
    </>
  )
}

function RepoShell({ title, route, children }: { title: string; route: Route; children: React.ReactNode }) {
  return (
    <main className="shell shell-repo">
      <header className="repo-header page-intro">
        <div>
          <p className="eyebrow">Repository</p>
          <h1>{title}</h1>
          <p className="page-subtitle">Source, history, and collaboration in one place.</p>
        </div>
        <span className="context-chip">Local forge</span>
      </header>
      {route.page === 'repo' && <RepoTabs route={route} />}
      {children}
    </main>
  )
}

function RepoTabs({ route }: { route: Extract<Route, { page: 'repo' }> }) {
  const base = `#/${route.owner}/${route.name}`
  const tabs: Array<[typeof route.tab, string]> = [
    ['code', 'Code'],
    ['commits', 'Commits'],
    ['issues', 'Issues'],
    ['pulls', 'Pull requests'],
  ]
  return (
    <nav className="repo-tabs" aria-label="Repository views">
      {tabs.map(([tab, label]) => (
        <a key={tab} className={`tab${route.tab === tab ? ' active' : ''}`} href={`${base}/${tab === 'code' ? 'tree/HEAD' : tab}`}>
          {label}
        </a>
      ))}
    </nav>
  )
}
