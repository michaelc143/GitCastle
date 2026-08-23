import { useEffect, useState } from 'react'
import { logout, me, User } from './api'
import { Icon } from './components/Icon'
import HomePage from './HomePage'
import LoginPage from './LoginPage'
import { IssuesList, IssueDetail } from './IssuesPage'
import { PullRequestsList, PullRequestDetail } from './PullsPage'
import { CodeBrowser, CommitDetail, CommitHistory } from './RepoViewPage'
import { navigate, Route, useRoute } from './router'
import { ThemeProvider, useTheme } from './ThemeContext'
import './styles.css'

export default function App() {
  return (
    <ThemeProvider>
      <AppInner />
    </ThemeProvider>
  )
}

function AppInner() {
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
    return (
      <div className="page-loading" role="status" aria-label="Loading">
        <span className="spinner" aria-hidden="true" />
      </div>
    )
  }

  return (
    <>
      <TopBar user={user} onLogout={handleLogout} />
      {route.page === 'home' && <HomePage user={user} onRequireLogin={() => navigate('/login')} />}
      {route.page === 'login' && !user && (
        <LoginPage
          onAuthenticated={(username) => {
            setUser({ id: 0, username, created_at: new Date().toISOString() })
            navigate('/')
          }}
          onGoHome={() => navigate('/')}
        />
      )}
      {route.page === 'repo' && (
        <RepoShell route={route}>
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
      <footer className="site-footer">
        <span>GitCastle — your code, your rules.</span>
      </footer>
    </>
  )
}

/* --- Top navigation --- */

function TopBar({ user, onLogout }: { user: User | null; onLogout: () => void }) {
  const { theme, toggleTheme } = useTheme()
  const nextTheme = theme === 'dark' ? 'light' : 'dark'

  return (
    <nav className="topbar">
      <div className="topbar-inner">
        <a className="brand" href="#/" aria-label="GitCastle home">
          <svg className="brand-mark" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
            {/* Castle rook mark */}
            <path
              d="M5 21V8l-1.5 0V4.5H7V6h2.5V4.5h5V6H17V4.5h3.5V8L19 8v13z"
              fill="currentColor"
              opacity="0.9"
            />
            <path d="M10 21v-5a2 2 0 0 1 4 0v5" fill="var(--bg)" />
          </svg>
          <span className="brand-name">GitCastle</span>
        </a>

        <div className="topbar-controls">
          <button
            type="button"
            className="icon-btn"
            onClick={toggleTheme}
            aria-label={`Switch to ${nextTheme} mode`}
            title={`Switch to ${nextTheme} mode`}
          >
            <Icon name={theme === 'dark' ? 'sun' : 'moon'} />
          </button>

          <span className="topbar-divider" aria-hidden="true" />

          {user ? (
            <div className="user-menu">
              <span className="avatar" title={user.username} aria-hidden="true">
                {user.username.charAt(0).toUpperCase()}
              </span>
              <span className="user-menu-name">{user.username}</span>
              <button type="button" className="btn btn-ghost btn-sm" onClick={onLogout}>
                <Icon name="logout" size={14} />
                Sign out
              </button>
            </div>
          ) : (
            <a className="btn btn-primary btn-sm" href="#/login">Sign in</a>
          )}
        </div>
      </div>
    </nav>
  )
}

/* --- Repository shell: header + tabs --- */

function RepoShell({ route, children }: { route: Route; children: React.ReactNode }) {
  if (route.page !== 'repo') return null
  return (
    <main className="shell shell-repo">
      <header className="repo-header">
        <div className="repo-header-text">
          <p className="eyebrow">Repository</p>
          <h1>
            <a href={`#/${route.owner}`} className="repo-owner-link">{route.owner}</a>
            <span className="repo-separator" aria-hidden="true">/</span>
            <span>{route.name}</span>
          </h1>
        </div>
        <CloneChip owner={route.owner} name={route.name} />
      </header>
      <RepoTabs route={route} />
      {children}
    </main>
  )
}

function CloneChip({ owner, name }: { owner: string; name: string }) {
  const [copied, setCopied] = useState(false)
  const cloneURL = `${window.location.origin}/git/${owner}/${name}.git`

  async function copy() {
    try {
      await navigator.clipboard.writeText(cloneURL)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      // Clipboard unavailable (permissions/insecure context); leave state.
    }
  }

  return (
    <div className="clone-chip" title="Clone URL (HTTP basic auth)">
      <input readOnly value={cloneURL} aria-label="Clone URL" onFocus={(e) => e.target.select()} />
      <button type="button" className="btn btn-secondary btn-sm" onClick={copy}>
        <Icon name={copied ? 'check' : 'link'} size={14} />
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

function RepoTabs({ route }: { route: Extract<Route, { page: 'repo' }> }) {
  const base = `#/${route.owner}/${route.name}`
  const tabs = [
    { key: 'code', label: 'Code', icon: 'code', href: `${base}/tree/HEAD` },
    { key: 'commits', label: 'Commits', icon: 'commit', href: `${base}/commits/HEAD` },
    { key: 'issues', label: 'Issues', icon: 'issue-open', href: `${base}/issues` },
    { key: 'pulls', label: 'Pull requests', icon: 'pull-request', href: `${base}/pulls` },
  ] as const

  return (
    <nav className="repo-tabs" aria-label="Repository views">
      {tabs.map(({ key, label, icon, href }) => (
        <a key={key} className={`tab${route.tab === key ? ' active' : ''}`} href={href}>
          <Icon name={icon} size={15} />
          {label}
        </a>
      ))}
    </nav>
  )
}
