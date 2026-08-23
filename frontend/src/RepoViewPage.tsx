import { useEffect, useState } from 'react'
import {
  BlobResponse,
  Commit,
  getBlob,
  getCommit,
  listCommits,
  listRefs,
  listTree,
  Ref,
  TreeEntry,
} from './api'
import { Icon } from './components/Icon'

/* Repository content views: code browser, commit history, commit detail. */

type Props = {
  owner: string
  name: string
}

type RevState = {
  rev: string
  refs: Ref[]
  head: string
}

function useRepoRefs(owner: string, name: string): RevState | null {
  const [state, setState] = useState<RevState | null>(null)
  useEffect(() => {
    let cancelled = false
    listRefs(owner, name)
      .then(({ refs, head }) => {
        if (!cancelled) setState({ rev: head || 'HEAD', refs: refs ?? [], head })
      })
      .catch(() => { if (!cancelled) setState({ rev: 'HEAD', refs: [], head: 'HEAD' }) })
    return () => { cancelled = true }
  }, [owner, name])
  return state
}

/* --- Code browser --- */

export function CodeBrowser({ owner, name, rev, filePath }: Props & { rev: string; filePath: string }) {
  const refState = useRepoRefs(owner, name)
  const activeRev = rev || refState?.rev || 'HEAD'
  const [entries, setEntries] = useState<TreeEntry[] | null>(null)
  const [blob, setBlob] = useState<BlobResponse | null>(null)
  const [error, setError] = useState('')
  const [emptyRepo, setEmptyRepo] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setEntries(null)
    setBlob(null)
    setError('')
    setEmptyRepo(false)
    setLoading(true)

    // Direct blob navigation: the router path after the rev is the file path.
    if (filePath) {
      getBlob(owner, name, activeRev, filePath)
        .then((response) => { if (!cancelled) setBlob(response) })
        .catch((reason) => { if (!cancelled) setError(reason.message) })
        .finally(() => { if (!cancelled) setLoading(false) })
    } else {
      listTree(owner, name, activeRev)
        .then((list) => { if (!cancelled) setEntries(list) })
        .catch((reason) => {
          if (!cancelled) {
            // A brand-new repository has no commits: rev resolution fails
            // with 404. That is not an error for the user; show guidance.
            setEmptyRepo(isNotFoundMessage(reason))
            setError('')
          }
        })
        .finally(() => { if (!cancelled) setLoading(false) })
    }
    return () => { cancelled = true }
  }, [owner, name, activeRev, filePath])

  return (
    <div className="content-stack">
      <RefBar owner={owner} name={name} state={refState} activeRev={activeRev} />
      <Breadcrumbs owner={owner} name={name} rev={activeRev} filePath={filePath} />

      {loading && <LoadingRows rows={6} />}
      {!loading && emptyRepo && (
        <div className="empty-repo-panel">
          <h3>This repository is empty</h3>
          <p>Push your first commit to see it here:</p>
          <pre className="clone-cmd">{`git remote add origin ${cloneURL(owner, name)}\ngit push -u origin main`}</pre>
        </div>
      )}

      {!loading && !emptyRepo && error && <p className="error" role="alert">{error}</p>}

      {!loading && !error && !emptyRepo && entries !== null && (
        <div className="tree-panel">
          <table className="tree-table">
            <tbody>
              {entries.length === 0 && (
                <tr><td className="empty-state">Empty repository — push some code.</td></tr>
              )}
              {/* Directories first, then files, both alphabetically. */}
              {[...entries].sort(sortEntries).map((entry) => (
                <tr key={`${entry.type}/${entry.path}`}>
                  <td className="type-cell">
                    <Icon name={entry.type === 'tree' ? 'folder' : 'file'} size={15} />
                  </td>
                  <td>
                    <a className="tree-link" href={`#/${owner}/${name}/tree/${encodeURIComponent(activeRev)}${filePath ? `/${filePath}` : ''}/${entry.path}`}>
                      {entry.path}
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!loading && !error && blob?.too_large && (
        <p className="empty-state">This file is too large to display.</p>
      )}
      {!loading && !error && filePath !== '' && blob?.content !== undefined && (
        <pre className="blob-view"><code>{blob.content}</code></pre>
      )}
    </div>
  )
}

function isNotFoundMessage(reason: unknown): boolean {
  return reason instanceof Error && /not found/i.test(reason.message)
}

function cloneURL(owner: string, name: string): string {
  return `${window.location.origin}/git/${owner}/${name}.git`
}

function sortEntries(a: TreeEntry, b: TreeEntry): number {
  if (a.type !== b.type) return a.type === 'tree' ? -1 : 1
  return a.path.localeCompare(b.path)
}

function LoadingRows({ rows }: { rows: number }) {
  return (
    <div className="skeleton-list" aria-hidden="true">
      {Array.from({ length: rows }, (_, index) => <div key={index} className="skeleton-row" />)}
    </div>
  )
}

/* --- Commit history --- */

export function CommitHistory({ owner, name, rev }: Props & { rev: string }) {
  const refState = useRepoRefs(owner, name)
  const activeRev = rev || refState?.rev || 'HEAD'
  const [commits, setCommits] = useState<Commit[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    listCommits(owner, name, activeRev)
      .then((list) => { if (!cancelled) setCommits(list) })
      .catch((reason) => { if (!cancelled) setError(reason.message) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [owner, name, activeRev])

  return (
    <div className="content-stack">
      <RefBar owner={owner} name={name} state={refState} activeRev={activeRev} />
      {loading && <LoadingRows rows={5} />}
      {!loading && error && <p className="error" role="alert">{error}</p>}
      {!loading && !error && commits.length === 0 && (
        <p className="empty-state">No commits on this branch yet.</p>
      )}
      {!loading && commits.length > 0 && (
        <ul className="commit-list">
          {commits.map((commit) => (
            <li className="commit-row" key={commit.hash}>
              <Icon name="commit" size={15} className="commit-icon" />
              <div className="commit-main">
                <a className="commit-message" href={`#/${owner}/${name}/commit/${commit.hash}`}>
                  {firstLine(commit.message)}
                </a>
                <span className="muted">
                  {commit.author} committed {relativeTime(commit.date)}
                </span>
              </div>
              <code className="hash-chip">{commit.hash.slice(0, 8)}</code>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

/* --- Commit detail + diff --- */

export function CommitDetail({ owner, name, hash }: Props & { hash: string }) {
  const [data, setData] = useState<{ commit: Commit; patch: string } | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    getCommit(owner, name, hash)
      .then((payload) => { if (!cancelled) setData(payload) })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : 'Load failed') })
    return () => { cancelled = true }
  }, [owner, name, hash])

  if (error) return <p className="error" role="alert">{error}</p>
  if (!data) return <LoadingRows rows={8} />

  const { commit, patch } = data
  return (
    <div className="content-stack">
      <p>
        <a className="back-link" href={`#/${owner}/${name}/commits/HEAD`}>
          <Icon name="branch" size={14} /> Back to history
        </a>
      </p>
      <header className="commit-detail-header">
        <h2>{firstLine(commit.message)}</h2>
        <p className="muted">
          {commit.author} committed {relativeTime(commit.date)} ·{' '}
          <code className="hash-chip">{commit.hash.slice(0, 8)}</code>
        </p>
      </header>
      {restOfMessage(commit.message) && (
        <pre className="blob-view commit-body">{restOfMessage(commit.message)}</pre>
      )}
      <DiffView patch={patch} />
    </div>
  )
}

type DiffStats = { additions: number; deletions: number }

export function DiffView({ patch }: { patch: string }) {
  const lines = patch.split('\n')
  const stats = diffStats(lines)

  return (
    <section className="diff-panel">
      <header className="diff-header">
        <Icon name="code" size={14} />
        <span>Diff</span>
        <span className="diff-stat">
          <span className="diff-add-text">+{stats.additions}</span>{' '}
          <span className="diff-del-text">−{stats.deletions}</span>
        </span>
      </header>
      <pre className="diff-view">
        {lines.map((line, index) => {
          let cls = 'diff-line'
          if (line.startsWith('+') && !line.startsWith('+++')) cls += ' diff-add'
          else if (line.startsWith('-') && !line.startsWith('---')) cls += ' diff-del'
          else if (line.startsWith('@@')) cls += ' diff-hunk'
          else if (line.startsWith('diff ') || line.startsWith('index ')) cls += ' diff-meta'
          return <div key={index} className={cls}>{line || ' '}</div>
        })}
      </pre>
    </section>
  )
}

function diffStats(lines: string[]): DiffStats {
  let additions = 0
  let deletions = 0
  for (const line of lines) {
    if (line.startsWith('+') && !line.startsWith('+++')) additions++
    else if (line.startsWith('-') && !line.startsWith('---')) deletions++
  }
  return { additions, deletions }
}

/* --- Shared pieces --- */

function RefBar({ owner, name, state, activeRev }: Props & { state: RevState | null; activeRev: string }) {
  if (!state) return null
  const allRefs = state.refs ?? []
  const branches = allRefs.filter((ref) => !ref.is_tag)
  const tags = allRefs.filter((ref) => ref.is_tag)
  return (
    <nav className="ref-bar" aria-label="Branches and tags">
      {branches.length + tags.length === 0 ? (
        <span className="muted">no branches yet</span>
      ) : (
        <>
          {branches.map((ref) => (
            <RefChip key={`b-${ref.name}`} owner={owner} name={name} ref_={ref} activeRev={activeRev} icon="branch" />
          ))}
          {tags.map((ref) => (
            <RefChip key={`t-${ref.name}`} owner={owner} name={name} ref_={ref} activeRev={activeRev} icon="tag" />
          ))}
        </>
      )}
    </nav>
  )
}

function RefChip({ owner, name, ref_, activeRev, icon }: {
  owner: string
  name: string
  ref_: Ref
  activeRev: string
  icon: 'branch' | 'tag'
}) {
  return (
    <a
      className={`ref-chip${ref_.name === activeRev ? ' active' : ''}`}
      href={`#/${owner}/${name}/tree/${encodeURIComponent(ref_.name)}`}
    >
      <Icon name={icon} size={13} />
      {ref_.name}
    </a>
  )
}

function Breadcrumbs({ owner, name, rev, filePath }: Props & { rev: string; filePath: string }) {
  if (!filePath) return null
  const segments = filePath.split('/').filter(Boolean)
  return (
    <nav className="breadcrumbs" aria-label="Path">
      <a href={`#/${owner}/${name}/tree/${encodeURIComponent(rev)}`}>{name}</a>
      {segments.map((segment, index) => {
        const partial = segments.slice(0, index + 1).join('/')
        const isLast = index === segments.length - 1
        return (
          <span key={partial}>
            <span className="crumb-separator" aria-hidden="true">/</span>
            {isLast ? <strong aria-current="page">{segment}</strong> : (
              <a href={`#/${owner}/${name}/tree/${encodeURIComponent(rev)}/${partial}`}>{segment}</a>
            )}
          </span>
        )
      })}
    </nav>
  )
}

/* --- Helpers --- */

function firstLine(message: string): string {
  return message.split('\n')[0] ?? message
}

function restOfMessage(message: string): string {
  const newline = message.indexOf('\n')
  return newline >= 0 ? message.slice(newline + 1).trim() : ''
}

export function relativeTime(dateString: string): string {
  const date = new Date(dateString)
  if (Number.isNaN(date.getTime())) return dateString
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} minute${minutes !== 1 ? 's' : ''} ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} hour${hours !== 1 ? 's' : ''} ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} day${days !== 1 ? 's' : ''} ago`
  return date.toLocaleDateString()
}
