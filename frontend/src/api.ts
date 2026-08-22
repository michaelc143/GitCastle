export type Repository = {
  id: number
  owner: string
  name: string
  path: string
  created_at: string
}

export type User = {
  id: number
  username: string
  created_at: string
}

export type Ref = {
  name: string
  hash: string
  is_tag: boolean
}

export type TreeEntry = {
  mode: string
  type: 'blob' | 'tree'
  hash: string
  path: string
}

export type Commit = {
  hash: string
  author: string
  email: string
  date: string
  message: string
  parents: string[]
}

export type BlobResponse = {
  content?: string
  too_large: boolean
}

type RepositoryListResponse = {
  repositories: Repository[]
}

type ErrorBody = { error?: string }

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init,
  })
  if (!response.ok) {
    if (response.status === 401) {
      throw new ApiError(401, 'Not signed in')
    }
    const body = (await response.json().catch(() => null)) as ErrorBody | null
    throw new ApiError(response.status, body?.error ?? 'Request failed')
  }
  return response.json() as Promise<T>
}

// --- auth ---

export function register(username: string, password: string): Promise<User> {
  return request<User>('/api/v1/register', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export async function login(username: string, password: string): Promise<User> {
  return request<User>('/api/v1/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export function logout(): Promise<void> {
  return request<{ status: string }>('/api/v1/logout', { method: 'POST' }).then(() => undefined)
}

export async function me(): Promise<User | null> {
  try {
    return await request<User>('/api/v1/me')
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) return null
    throw error
  }
}

// --- repositories ---

export async function listRepositories(): Promise<Repository[]> {
  const response = await request<RepositoryListResponse>('/api/v1/repositories')
  return response.repositories
}

export function createRepository(owner: string, name: string): Promise<Repository> {
  return request<Repository>('/api/v1/repositories', {
    method: 'POST',
    body: JSON.stringify({ owner, name }),
  })
}

// --- repository content ---

function repoPath(owner: string, name: string, suffix: string): string {
  return `/api/v1/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(name)}${suffix}`
}

export async function listRefs(owner: string, name: string): Promise<{ refs: Ref[]; head: string }> {
  return request(repoPath(owner, name, '/refs'))
}

export async function listTree(owner: string, name: string, rev: string, path = ''): Promise<TreeEntry[]> {
  const query = path ? `?path=${encodeURIComponent(path)}` : ''
  const response = await request<{ entries: TreeEntry[] }>(repoPath(owner, name, `/tree/${encodeURIComponent(rev)}${query}`))
  return response.entries
}

export async function getBlob(owner: string, name: string, rev: string, path: string): Promise<BlobResponse> {
  return request(repoPath(owner, name, `/blob/${encodeURIComponent(rev)}?path=${encodeURIComponent(path)}`))
}

export async function listCommits(owner: string, name: string, rev: string): Promise<Commit[]> {
  const response = await request<{ commits: Commit[] }>(repoPath(owner, name, `/commits/${encodeURIComponent(rev)}`))
  return response.commits
}

export async function getCommit(owner: string, name: string, hash: string): Promise<{ commit: Commit; patch: string }> {
  return request(repoPath(owner, name, `/commit/${encodeURIComponent(hash)}`))
}

// --- collaboration (Phase 3) ---

export type Issue = {
  number: number
  title: string
  body: string
  author: string
  state: 'open' | 'closed'
  created_at: string
  updated_at: string
}

export type Comment = {
  id: number
  author: string
  body: string
  created_at: string
}

export type PullRequest = {
  number: number
  title: string
  body: string
  author: string
  state: 'open' | 'merged' | 'closed'
  source_branch: string
  target_branch: string
  merge_commit?: string | null
  created_at: string
  updated_at: string
}

export type MergeCheck = {
  mergable: boolean
  blockers: string[]
  required_approvals: number
  current_approvals: number
}

export type Review = {
  reviewer: string
  verdict: 'approved' | 'changes_requested' | 'commented'
  body: string
  created_at: string
}

export type BranchProtection = {
  branch: string
  required_approvals: number
  allow_force_push: boolean
}

export async function listIssues(owner: string, name: string, state = ''): Promise<Issue[]> {
  const query = state ? `?state=${state}` : ''
  const response = await request<{ issues: Issue[] }>(repoPath(owner, name, `/issues${query}`))
  return response.issues
}

export function createIssue(owner: string, name: string, title: string, body: string): Promise<Issue> {
  return request(repoPath(owner, name, '/issues'), {
    method: 'POST',
    body: JSON.stringify({ title, body }),
  })
}

export function getIssue(owner: string, name: string, number: number): Promise<Issue> {
  return request(repoPath(owner, name, `/issues/${number}`))
}

export function setIssueState(owner: string, name: string, number: number, state: string): Promise<Issue> {
  return request(repoPath(owner, name, `/issues/${number}`), {
    method: 'PATCH',
    body: JSON.stringify({ state }),
  })
}

export async function listComments(owner: string, name: string, subject: 'issues' | 'pulls', number: number): Promise<Comment[]> {
  const response = await request<{ comments: Comment[] }>(repoPath(owner, name, `/${subject}/${number}/comments`))
  return response.comments
}

export function addComment(owner: string, name: string, subject: 'issues' | 'pulls', number: number, body: string): Promise<Comment> {
  return request(repoPath(owner, name, `/${subject}/${number}/comments`), {
    method: 'POST',
    body: JSON.stringify({ body }),
  })
}

export async function listPullRequests(owner: string, name: string, state = ''): Promise<PullRequest[]> {
  const query = state ? `?state=${state}` : ''
  const response = await request<{ pull_requests: PullRequest[] }>(repoPath(owner, name, `/pulls${query}`))
  return response.pull_requests
}

export function createPullRequest(owner: string, name: string, input: {
  title: string; body: string; source_branch: string; target_branch: string
}): Promise<PullRequest> {
  return request(repoPath(owner, name, '/pulls'), {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function getPullRequest(owner: string, name: string, number: number): Promise<{ pull_request: PullRequest; merge_check: MergeCheck }> {
  return request(repoPath(owner, name, `/pulls/${number}`))
}

export function mergePullRequest(owner: string, name: string, number: number): Promise<PullRequest> {
  return request(repoPath(owner, name, `/pulls/${number}/merge`), { method: 'POST' })
}

export function putReview(owner: string, name: string, number: number, verdict: string, body: string): Promise<Review> {
  return request(repoPath(owner, name, `/pulls/${number}/review`), {
    method: 'PUT',
    body: JSON.stringify({ verdict, body }),
  })
}

export async function listReviews(owner: string, name: string, number: number): Promise<Review[]> {
  const response = await request<{ reviews: Review[] }>(repoPath(owner, name, `/pulls/${number}/reviews`))
  return response.reviews
}

export function setBranchProtection(owner: string, name: string, branch: string, requiredApprovals: number): Promise<BranchProtection> {
  return request(repoPath(owner, name, `/branches/${encodeURIComponent(branch)}/protection`), {
    method: 'PUT',
    body: JSON.stringify({ required_approvals: requiredApprovals }),
  })
}
