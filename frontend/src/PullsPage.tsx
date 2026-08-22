import { useEffect, useState } from 'react'
import {
  addComment,
  Comment,
  createPullRequest,
  getPullRequest,
  listComments,
  listPullRequests,
  listRefs,
  MergeCheck,
  mergePullRequest,
  putReview,
  PullRequest,
  Review,
  listReviews,
  Ref,
} from './api'

type Props = { owner: string; name: string; signedIn: boolean }

export function PullRequestsList({ owner, name, signedIn }: Props) {
  const [pulls, setPulls] = useState<PullRequest[]>([])
  const [refs, setRefs] = useState<Ref[]>([])
  const [source, setSource] = useState('')
  const [target, setTarget] = useState('main')
  const [title, setTitle] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    listPullRequests(owner, name)
      .then(setPulls)
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
    listRefs(owner, name)
      .then(({ refs: loaded }) => setRefs(loaded.filter((ref) => !ref.is_tag)))
      .catch(() => setRefs([]))
  }, [owner, name])

  async function submit() {
    if (!title.trim() || !source || !target) return
    try {
      const pr = await createPullRequest(owner, name, {
        title: title.trim(), body: '', source_branch: source, target_branch: target,
      })
      setPulls((current) => [pr, ...current])
      setTitle('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Create failed')
    }
  }

  return (
    <div>
      <div className="section-heading"><h2>Pull requests</h2></div>
      {signedIn && refs.length > 1 && (
        <div className="card stack">
          <input placeholder="Title" value={title} onChange={(event) => setTitle(event.target.value)} />
          <div className="branch-pair">
            <select value={source} onChange={(event) => setSource(event.target.value)} aria-label="Source branch">
              <option value="">from…</option>
              {refs.map((ref) => <option key={ref.name} value={ref.name}>{ref.name}</option>)}
            </select>
            <span className="muted">→</span>
            <select value={target} onChange={(event) => setTarget(event.target.value)} aria-label="Target branch">
              {refs.map((ref) => <option key={ref.name} value={ref.name}>{ref.name}</option>)}
            </select>
          </div>
          <button type="button" onClick={submit} disabled={!title.trim() || !source || source === target}>
            Open pull request
          </button>
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      <ul className="issue-list">
        {pulls.map((pr) => (
          <li key={pr.number} className="commit-row">
            <span className={`state-pill ${pr.state}`}>{pr.state}</span>
            <span>#{pr.number} <strong>{pr.title}</strong> <span className="muted">({pr.source_branch} → {pr.target_branch})</span></span>
            <span className="muted">{pr.author}</span>
          </li>
        ))}
      </ul>
      {!error && pulls.length === 0 && <p className="empty-state">No pull requests yet.</p>}
    </div>
  )
}

export function PullRequestDetail({ owner, name, number, signedIn }: Props & { number: number }) {
  const [data, setData] = useState<{ pull_request: PullRequest; merge_check: MergeCheck } | null>(null)
  const [reviews, setReviews] = useState<Review[]>([])
  const [comments, setComments] = useState<Comment[]>([])
  const [draft, setDraft] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  function reload() {
    Promise.all([
      getPullRequest(owner, name, number),
      listReviews(owner, name, number),
      listComments(owner, name, 'pulls', number),
    ])
      .then(([detail, loadedReviews, loadedComments]) => {
        setData(detail); setReviews(loadedReviews); setComments(loadedComments)
      })
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
  }

  useEffect(reload === undefined ? () => {} : () => { reload() }, [owner, name, number])

  async function review(verdict: 'approved' | 'changes_requested' | 'commented') {
    setBusy(true)
    try {
      await putReview(owner, name, number, verdict, '')
      reload()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Review failed')
    } finally {
      setBusy(false)
    }
  }

  async function doMerge() {
    setBusy(true)
    try {
      await mergePullRequest(owner, name, number)
      reload()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Merge failed')
    } finally {
      setBusy(false)
    }
  }

  async function postComment() {
    if (!draft.trim()) return
    try {
      const comment = await addComment(owner, name, 'pulls', number, draft.trim())
      setComments((current) => [...current, comment])
      setDraft('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Post failed')
    }
  }

  if (error) return <p className="error" role="alert">{error}</p>
  if (!data) return <p className="empty-state">Loading…</p>

  const { pull_request: pr, merge_check: check } = data
  return (
    <div>
      <h2>#{pr.number} {pr.title}</h2>
      <p className="muted">
        <span className={`state-pill ${pr.state}`}>{pr.state}</span>{' '}
        {pr.source_branch} → {pr.target_branch} · opened by {pr.author}
      </p>

      <div className="card">
        <h3>Merge readiness</h3>
        {check.mergable ? (
          <>
            <p>✅ All checks passed ({check.current_approvals}/{check.required_approvals} approvals).</p>
            {signedIn && pr.state === 'open' && (
              <button type="button" onClick={doMerge} disabled={busy}>Merge pull request</button>
            )}
          </>
        ) : (
          <ul>
            {check.blockers.map((blocker) => <li key={blocker}>⛔ {blocker}</li>)}
          </ul>
        )}
      </div>

      <div className="card">
        <h3>Reviews</h3>
        <ul className="review-list">
          {reviews.map((entry) => (
            <li key={entry.reviewer}>
              <strong>{entry.reviewer}</strong> — <span className={`state-pill ${entry.verdict === 'approved' ? 'open' : 'closed'}`}>{entry.verdict.replace('_', ' ')}</span>
            </li>
          ))}
        </ul>
        {signedIn && pr.state === 'open' && (
          <div className="review-actions">
            <button type="button" onClick={() => review('approved')} disabled={busy}>Approve</button>
            <button type="button" onClick={() => review('changes_requested')} disabled={busy}>Request changes</button>
            <button type="button" onClick={() => review('commented')} disabled={busy}>Comment review</button>
          </div>
        )}
      </div>

      <h3>Conversation</h3>
      <ul className="comment-list">
        {comments.map((comment) => (
          <li key={comment.id} className="commit-row">
            <strong>{comment.author}</strong>
            <p>{comment.body}</p>
          </li>
        ))}
      </ul>
      {signedIn && (
        <div className="card stack">
          <textarea value={draft} onChange={(event) => setDraft(event.target.value)} rows={3} placeholder="Write a comment…" />
          <button type="button" onClick={postComment} disabled={!draft.trim()}>Comment</button>
        </div>
      )}
    </div>
  )
}
