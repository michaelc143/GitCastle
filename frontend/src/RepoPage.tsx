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
      .then(({ refs, head }) => { if (!cancelled) setState({ rev: head || 'HEAD', refs, head }) })
      .catch(() => { if (!cancelled) setState({ rev: 'HEAD', refs: [], head: 'HEAD' }) })
    return () => { cancelled = true }
  }, [owner, name])
  return state
}

export function CodeBrowser({ owner, name, rev, filePath }: Props & { rev: string; filePath: string }) {
  const refState = useRepoRefs(owner, name)
  const activeRev = rev || refState?.rev || 'HEAD'
  const [entries, setEntries] = useState<TreeEntry[] | null>(null)
  const [blob, setBlob] = useState<BlobResponse | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setEntries(null); setBlob(null); setError('')
    if (filePath) {
      getBlob(owner, name, activeRev, filePath)
        .then((response) => { if (!cancelled) setBlob(response) })
        .catch((reason) => { if (!cancelled) setError(reason.message) })
    } else {
      listTree(owner, name, activeRev)
        .then((list) => { if (!cancelled) setEntries(list) })
        .catch((reason) => { if (!cancelled) setError(reason.message) })
    }
    return () => { cancelled = true }
  }, [owner, name, activeRev, filePath])

  return (
    <div>
      <RefBar owner={owner} name={name} state={refState} activeRev={activeRev} />
      <Breadcrumbs owner={owner} name={name} rev={activeRev} filePath={filePath} />
      {error && <p className="error" role="alert">{error}</p>}
      {!error && filePath === '' && entries && (
        <table className="tree-table">
          <tbody>
            {entries.map((entry) => (
              <tr key={`${entry.type}/${entry.path}`}>
                <td className={`type-cell type-${entry.type}`} aria-hidden="true">{entry.type === 'tree' ? 'DIR' : 'FILE'}</td>
                <td>
                  <a href={`#/${owner}/${name}/tree/${activeRev}${filePath ? `/${filePath}` : ''}/${entry.path}`}>
                    {entry.path}
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {!error && filePath !== '' && blob?.too_large && (
        <p className="empty-state">This file is too large to display.</p>
      )}
      {!error && filePath !== '' && blob?.content !== undefined && (
        <pre className="blob-view"><code>{blob.content}</code></pre>
      )}
    </div>
  )
}

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
    <div>
      <RefBar owner={owner} name={name} state={refState} activeRev={activeRev} />
      {loading && <p className="empty-state">Loading history…</p>}
      {error && <p className="error" role="alert">{error}</p>}
      <ul className="commit-list">
        {commits.map((commit) => (
          <li className="commit-row" key={commit.hash}>
            <a className="commit-message" href={`#/${owner}/${name}/commit/${commit.hash}`}>
              {firstLine(commit.message)}
            </a>
            <span className="muted">
              {commit.author} · {new Date(commit.date).toLocaleString()} ·{' '}
              <code>{commit.hash.slice(0, 8)}</code>
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

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
  if (!data) return <p className="empty-state">Loading commit…</p>

  const { commit, patch } = data
  return (
    <div>
      <p><a href={`#/${owner}/${name}/commits/HEAD`}>← Back to history</a></p>
      <h2>{firstLine(commit.message)}</h2>
      <p className="muted">
        {commit.author} committed on {new Date(commit.date).toLocaleString()} ·{' '}
        <code>{commit.hash.slice(0, 8)}</code>
      </p>
      {commit.message.includes('\n') && (
        <pre className="commit-body">{restOfMessage(commit.message)}</pre>
      )}
      <DiffView patch={patch} />
    </div>
  )
}

// DiffView renders a unified diff with add/remove line highlighting.
export function DiffView({ patch }: { patch: string }) {
  const lines = patch.split('\n')
  return (
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
  )
}

function RefBar({ owner, name, state, activeRev }: Props & { state: RevState | null; activeRev: string }) {
  if (!state) return null
  const branches = state.refs.filter((ref) => !ref.is_tag)
  const tags = state.refs.filter((ref) => ref.is_tag)
  return (
    <nav className="ref-bar" aria-label="Branches and tags">
      {[...branches, ...tags].length === 0 ? (
        <span className="muted">no branches yet</span>
      ) : [...branches, ...tags].map((ref) => (
        <a
          key={ref.name}
          className={`ref-chip${ref.name === activeRev ? ' active' : ''}`}
          href={`#/${owner}/${name}/tree/${encodeURIComponent(ref.name)}`}
        >
          <span className="ref-symbol" aria-hidden="true">{ref.is_tag ? 'TAG' : 'BR'}</span>{ref.name}
        </a>
      ))}
    </nav>
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
            {' / '}
            {isLast ? (
              <strong>{segment}</strong>
            ) : (
              <a href={`#/${owner}/${name}/tree/${encodeURIComponent(rev)}/${partial}`}>{segment}</a>
            )}
          </span>
        )
      })}
    </nav>
  )
}

function firstLine(message: string): string {
  return message.split('\n')[0] ?? message
}

function restOfMessage(message: string): string {
  const newline = message.indexOf('\n')
  return newline >= 0 ? message.slice(newline + 1).trim() : ''
}
