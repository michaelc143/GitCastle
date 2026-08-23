import { useEffect, useState } from 'react'
import {
  addComment,
  createIssue,
  Issue,
  listComments,
  listIssues,
  Comment,
  setIssueState,
} from './api'
import { Icon } from './components/Icon'
import { relativeTime } from './RepoViewPage'

type Props = { owner: string; name: string; signedIn: boolean }

export function IssuesList({ owner, name, signedIn }: Props) {
  const [issues, setIssues] = useState<Issue[]>([])
  const [showClosed, setShowClosed] = useState(false)
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    listIssues(owner, name, showClosed ? 'closed' : 'open')
      .then((list) => { if (!cancelled) { setIssues(list); setError('') } })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : 'Load failed') })
    return () => { cancelled = true }
  }, [owner, name, showClosed])

  async function submit() {
    if (!title.trim()) return
    setCreating(true)
    try {
      const issue = await createIssue(owner, name, title.trim(), body.trim())
      setIssues((current) => [issue, ...current])
      setTitle('')
      setBody('')
      setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Create failed')
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="content-stack">
      <div className="list-toolbar">
        <h2>
          <Icon name={showClosed ? 'issue-closed' : 'issue-open'} size={16} />
          {issues.length} {showClosed ? 'closed' : 'open'} issue{issues.length !== 1 ? 's' : ''}
        </h2>
        <label className="toggle">
          <input type="checkbox" checked={showClosed} onChange={(event) => setShowClosed(event.target.checked)} />
          Show closed
        </label>
      </div>

      {signedIn && (
        <div className="card stack">
          <input
            placeholder="Issue title"
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            aria-label="Issue title"
          />
          <textarea
            placeholder="Describe the problem or idea (optional)"
            value={body}
            onChange={(event) => setBody(event.target.value)}
            rows={3}
            aria-label="Issue description"
          />
          <div className="form-actions">
            <button type="button" className="btn btn-primary" onClick={submit} disabled={!title.trim() || creating}>
              {creating ? 'Creating…' : 'Open issue'}
            </button>
          </div>
        </div>
      )}

      {error && <p className="error" role="alert">{error}</p>}

      {!error && issues.length === 0 ? (
        <p className="empty-state">No {showClosed ? 'closed' : 'open'} issues.</p>
      ) : (
        <ul className="item-list">
          {issues.map((issue) => (
            <li key={issue.number} className="item-row">
              <span className={`state-pill ${issue.state === 'open' ? 'is-open' : 'is-done'}`}>
                <Icon name={issue.state === 'open' ? 'issue-open' : 'issue-closed'} size={14} />
                {issue.state}
              </span>
              <a className="item-title" href={`#/${owner}/${name}/issues/${issue.number}`}>
                {issue.title}
              </a>
              <span className="item-meta">#{issue.number} · {issue.author} · opened {relativeTime(issue.created_at)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export function IssueDetail({ owner, name, number, signedIn }: Props & { number: number }) {
  const [issue, setIssue] = useState<Issue | null>(null)
  const [comments, setComments] = useState<Comment[]>([])
  const [draft, setDraft] = useState('')
  const [posting, setPosting] = useState(false)
  const [error, setError] = useState('')

  function load() {
    Promise.all([
      import('./api').then((m) => m.getIssue(owner, name, number)),
      listComments(owner, name, 'issues', number),
    ])
      .then(([loadedIssue, loadedComments]) => { setIssue(loadedIssue); setComments(loadedComments); setError('') })
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
  }

  useEffect(load, [owner, name, number])

  async function postComment() {
    if (!draft.trim()) return
    setPosting(true)
    try {
      const comment = await addComment(owner, name, 'issues', number, draft.trim())
      setComments((current) => [...current, comment])
      setDraft('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Post failed')
    } finally {
      setPosting(false)
    }
  }

  async function toggleState() {
    if (!issue) return
    try {
      setIssue(await setIssueState(owner, name, number, issue.state === 'open' ? 'closed' : 'open'))
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Update failed')
    }
  }

  if (error && !issue) return <p className="error" role="alert">{error}</p>
  if (!issue) return (
    <div className="skeleton-list" aria-hidden="true">
      <div className="skeleton-row" /><div className="skeleton-row" />
    </div>
  )

  return (
    <div className="content-stack">
      <header className="subject-header">
        <h2>
          {issue.title} <span className="subject-number">#{issue.number}</span>
        </h2>
        <p className="muted subject-meta">
          <span className={`state-pill ${issue.state === 'open' ? 'is-open' : 'is-done'}`}>
            <Icon name={issue.state === 'open' ? 'issue-open' : 'issue-closed'} size={14} />
            {issue.state}
          </span>{' '}
          opened by {issue.author} {relativeTime(issue.created_at)}
          {signedIn && (
            <>
              {' · '}
              <button type="button" className="link" onClick={toggleState}>
                {issue.state === 'open' ? 'Close issue' : 'Reopen'}
              </button>
            </>
          )}
        </p>
      </header>

      {error && <p className="error" role="alert">{error}</p>}
      {issue.body && <pre className="blob-view subject-body">{issue.body}</pre>}

      <section aria-label="Comments">
        <h3 className="section-title">Conversation</h3>
        {comments.length === 0 ? (
          <p className="empty-state">No comments yet.</p>
        ) : (
          <ul className="comment-list">
            {comments.map((comment) => (
              <li key={comment.id} className="comment-card">
                <header>
                  <span className="avatar avatar-sm" aria-hidden="true">{comment.author.charAt(0).toUpperCase()}</span>
                  <strong>{comment.author}</strong>
                  <span className="muted">commented {relativeTime(comment.created_at)}</span>
                </header>
                <p>{comment.body}</p>
              </li>
            ))}
          </ul>
        )}
      </section>

      {signedIn && (
        <div className="card stack">
          <textarea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            rows={3}
            placeholder="Write a comment…"
            aria-label="New comment"
          />
          <div className="form-actions">
            <button type="button" className="btn btn-primary" onClick={postComment} disabled={!draft.trim() || posting}>
              Comment
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
