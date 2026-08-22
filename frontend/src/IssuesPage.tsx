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

type Props = { owner: string; name: string; signedIn: boolean }

export function IssuesList({ owner, name, signedIn }: Props) {
  const [issues, setIssues] = useState<Issue[]>([])
  const [showClosed, setShowClosed] = useState(false)
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    listIssues(owner, name, showClosed ? 'closed' : 'open')
      .then(setIssues)
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
  }, [owner, name, showClosed])

  async function submit() {
    if (!title.trim()) return
    try {
      const issue = await createIssue(owner, name, title.trim(), body.trim())
      setIssues((current) => [issue, ...current])
      setTitle('')
      setBody('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Create failed')
    }
  }

  return (
    <div>
      <div className="section-heading">
        <h2>Issues</h2>
        <label className="toggle">
          <input type="checkbox" checked={showClosed} onChange={(event) => setShowClosed(event.target.checked)} />
          show closed
        </label>
      </div>
      {signedIn && (
        <div className="card stack">
          <input placeholder="Title" value={title} onChange={(event) => setTitle(event.target.value)} />
          <textarea placeholder="Details (optional)" value={body} onChange={(event) => setBody(event.target.value)} rows={3} />
          <button type="button" onClick={submit} disabled={!title.trim()}>New issue</button>
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      <ul className="issue-list">
        {issues.map((issue) => (
          <li key={issue.number} className="commit-row">
            <span className={`state-pill ${issue.state}`}>{issue.state}</span>
            <span>#{issue.number} <strong>{issue.title}</strong></span>
            <span className="muted">{issue.author}</span>
          </li>
        ))}
      </ul>
      {!error && issues.length === 0 && <p className="empty-state">No {showClosed ? 'closed' : 'open'} issues.</p>}
    </div>
  )
}

export function IssueDetail({ owner, name, number, signedIn }: Props & { number: number }) {
  const [issue, setIssue] = useState<Issue | null>(null)
  const [comments, setComments] = useState<Comment[]>([])
  const [draft, setDraft] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([import('./api').then((m) => m.getIssue(owner, name, number)), listComments(owner, name, 'issues', number)])
      .then(([loadedIssue, loadedComments]) => { setIssue(loadedIssue); setComments(loadedComments) })
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
  }, [owner, name, number])

  async function postComment() {
    if (!draft.trim()) return
    try {
      const comment = await addComment(owner, name, 'issues', number, draft.trim())
      setComments((current) => [...current, comment])
      setDraft('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Post failed')
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

  if (error) return <p className="error" role="alert">{error}</p>
  if (!issue) return <p className="empty-state">Loading…</p>

  return (
    <div>
      <h2>#{issue.number} {issue.title}</h2>
      <p className="muted">
        <span className={`state-pill ${issue.state}`}>{issue.state}</span> opened by {issue.author}
        {' · '}
        {new Date(issue.created_at).toLocaleString()}
        {signedIn && (
          <>
            {' · '}
            <button type="button" className="link" onClick={toggleState}>
              {issue.state === 'open' ? 'Close issue' : 'Reopen'}
            </button>
          </>
        )}
      </p>
      {issue.body && <pre className="blob-view">{issue.body}</pre>}
      <h3>Comments</h3>
      <ul className="comment-list">
        {comments.map((comment) => (
          <li key={comment.id} className="commit-row">
            <strong>{comment.author}</strong>
            <p>{comment.body}</p>
          </li>
        ))}
      </ul>
      {comments.length === 0 && <p className="empty-state">No comments yet.</p>}
      {signedIn && (
        <div className="card stack">
          <textarea value={draft} onChange={(event) => setDraft(event.target.value)} rows={3} placeholder="Write a comment…" />
          <button type="button" onClick={postComment} disabled={!draft.trim()}>Comment</button>
        </div>
      )}
    </div>
  )
}
