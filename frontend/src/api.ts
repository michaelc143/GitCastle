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
