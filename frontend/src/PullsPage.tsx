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
import { Icon } from './components/Icon'
import { relativeTime } from './RepoViewPage'

type Props = { owner: string; name: string; signedIn: boolean }

export function PullRequestsList({ owner, name, signedIn }: Props) {
  const [pulls, setPulls] = useState<PullRequest[]>([])
  const [refs, setRefs] = useState<Ref[]>([])
  const [source, setSource] = useState('')
  const [target, setTarget] = useState('main')
  const [title, setTitle] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')

  function load() {
    listPullRequests(owner, name)
      .then(setPulls)
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
    listRefs(owner, name)
      .then(({ refs: loaded }) => setRefs(loaded.filter((ref) => !ref.is_tag)))
      .catch(() => setRefs([]))
  }

  useEffect(load, [owner, name])

  async function submit() {
    if (!title.trim() || !source || !target) return
    setCreating(true)
    try {
      const pr = await createPullRequest(owner, name, {
        title: title.trim(), body: '', source_branch: source, target_branch: target,
      })
      setPulls((current) => [pr, ...current])
      setTitle('')
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
          <Icon name="pull-request" size={16} />
          {pulls.length} pull request{pulls.length !== 1 ? 's' : ''}
        </h2>
      </div>

      {signedIn && refs.length > 0 && (
        <div className="card stack">
          <h3 className="section-title">Propose a change</h3>
          <input
            placeholder="Pull request title"
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            aria-label="Pull request title"
          />
          <div className="branch-pair">
            <select value={source} onChange={(event) => setSource(event.target.value)} aria-label="Source branch">
              <option value="">from…</option>
              {refs.map((ref) => <option key={ref.name} value={ref.name}>{ref.name}</option>)}
            </select>
            <Icon name="pull-request" size={15} className="branch-arrow" />
            <select value={target} onChange={(event) => setTarget(event.target.value)} aria-label="Target branch">
              {refs.map((ref) => <option key={ref.name} value={ref.name}>{ref.name}</option>)}
            </select>
          </div>
          <div className="form-actions">
            <button
              type="button"
              className="btn btn-primary"
              onClick={submit}
              disabled={!title.trim() || !source || source === target || creating}
            >
              {creating ? 'Opening…' : 'Open pull request'}
            </button>
          </div>
        </div>
      )}

      {error && <p className="error" role="alert">{error}</p>}

      {!error && pulls.length === 0 ? (
        <p className="empty-state">No pull requests yet.</p>
      ) : (
        <ul className="item-list">
          {pulls.map((pr) => (
            <li key={pr.number} className="item-row">
              <span className={`state-pill ${pr.state === 'open' ? 'is-open' : pr.state === 'merged' ? 'is-merged' : 'is-done'}`}>
                <Icon name={pr.state === 'merged' ? 'merged' : 'pull-request'} size={14} />
                {pr.state}
              </span>
              <a className="item-title" href={`#/${owner}/${name}/pulls/${pr.number}`}>
                {pr.title}
              </a>
              <span className="item-meta">
                #{pr.number} · {pr.source_branch} → {pr.target_branch} · {pr.author} · {relativeTime(pr.created_at)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

const VERDICTS = [
  { verdict: 'approved', label: 'Approve', icon: 'check', className: 'btn-approve' },
  { verdict: 'changes_requested', label: 'Request changes', icon: 'x', className: 'btn-request' },
  { verdict: 'commented', label: 'Comment review', icon: 'commit', className: 'btn-secondary' },
] as const

export function PullRequestDetail({ owner, name, number, signedIn }: Props & { number: number }) {
  const [data, setData] = useState<{ pull_request: PullRequest; merge_check: MergeCheck } | null>(null)
  const [reviews, setReviews] = useState<Review[]>([])
  const [comments, setComments] = useState<Comment[]>([])
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  function load() {
    Promise.all([
      getPullRequest(owner, name, number),
      listReviews(owner, name, number),
      listComments(owner, name, 'pulls', number),
    ])
      .then(([detail, loadedReviews, loadedComments]) => {
        setData(detail); setReviews(loadedReviews); setComments(loadedComments); setError('')
      })
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Load failed'))
  }

  useEffect(load, [owner, name, number])

  async function review(verdict: string) {
    setBusy(true)
    try {
      await putReview(owner, name, number, verdict, '')
      load()
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
      load()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Merge failed')
    } finally {
      setBusy(false)
    }
  }

  async function postComment() {
    if (!draft.trim()) return
    setBusy(true)
    try {
      const comment = await addComment(owner, name, 'pulls', number, draft.trim())
      setComments((current) => [...current, comment])
      setDraft('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Post failed')
    } finally {
      setBusy(false)
    }
  }

  if (error && !data) return <p className="error" role="alert">{error}</p>
  if (!data) return (
    <div className="skeleton-list" aria-hidden="true">
      <div className="skeleton-row" /><div className="skeleton-row" /><div className="skeleton-row" />
    </div>
  )

  const { pull_request: pr, merge_check: check } = data
  const isOpen = pr.state === 'open'

  return (
    <div className="content-stack">
      <header className="subject-header">
        <h2>
          {pr.title} <span className="subject-number">#{pr.number}</span>
        </h2>
        <p className="muted subject-meta">
          <span className={`state-pill ${pr.state === 'open' ? 'is-open' : pr.state === 'merged' ? 'is-merged' : 'is-done'}`}>
            <Icon name={pr.state === 'merged' ? 'merged' : 'pull-request'} size={14} />
            {pr.state}
          </span>{' '}
          <code className="branch-ref">{pr.source_branch}</code> → <code className="branch-ref">{pr.target_branch}</code>
          {' '}· opened by {pr.author} {relativeTime(pr.created_at)}
        </p>
      </header>

      {error && <p className="error" role="alert">{error}</p>}

      {/* Merge readiness gate */}
      <section className={`merge-gate ${check.mergable && isOpen ? 'is-ready' : 'is-blocked'}`} aria-live="polite">
        <div className="merge-gate-icon" aria-hidden="true">
          <Icon name={check.mergable && isOpen ? 'check' : 'shield'} size={18} />
        </div>
        <div className="merge-gate-body">
          <strong>
            {!isOpen
              ? `This pull request is ${pr.state}.`
              : check.mergable
                ? 'All checks passed — ready to merge.'
                : 'Merge blocked:'}
          </strong>
          {!isOpen ? null : check.mergable ? (
            <p className="muted">{check.current_approvals}/{check.required_approvals} required approvals satisfied.</p>
          ) : (
            <ul>
              {check.blockers.map((blocker) => <li key={blocker}>{blocker}</li>)}
            </ul>
          )}
        </div>
        {isOpen && check.mergable && signedIn && (
          <button type="button" className="btn btn-primary" onClick={doMerge} disabled={busy}>
            <Icon name="merged" size={14} />
            Merge pull request
          </button>
        )}
      </section>

      {/* Reviews */}
      <section className="card" aria-label="Reviews">
        <h3 className="section-title">Reviews ({reviews.length})</h3>
        {reviews.length === 0 ? (
          <p className="empty-state">No reviews yet.</p>
        ) : (
          <ul className="review-list">
            {reviews.map((entry) => (
              <li key={entry.reviewer} className="review-row">
                <span className="avatar avatar-sm" aria-hidden="true">{entry.reviewer.charAt(0).toUpperCase()}</span>
                <strong>{entry.reviewer}</strong>
                <span className={`verdict verdict-${entry.verdict}`}>
                  <Icon name={entry.verdict === 'approved' ? 'check' : entry.verdict === 'changes_requested' ? 'x' : 'commit'} size={13} />
                  {entry.verdict.replace('_', ' ')}
                </span>
                <span className="muted">{relativeTime(entry.created_at)}</span>
              </li>
            ))}
          </ul>
        )}
        {signedIn && isOpen && (
          <div className="review-actions">
            {VERDICTS.map(({ verdict, label, icon, className }) => (
              <button key={verdict} type="button" className={`btn btn-sm ${className}`} onClick={() => review(verdict)} disabled={busy}>
                <Icon name={icon as 'check'} size={14} />
                {label}
              </button>
            ))}
          </div>
        )}
      </section>

      {/* Conversation */}
      <section aria-label="Conversation">
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
            <button type="button" className="btn btn-primary" onClick={postComment} disabled={!draft.trim() || busy}>
              Comment
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
