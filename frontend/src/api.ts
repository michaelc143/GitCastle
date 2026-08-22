export type Repository = {
  id: number
  owner: string
  name: string
  path: string
  created_at: string
}

type RepositoryListResponse = {
  repositories: Repository[]
}

async function request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init,
  })
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? 'Request failed')
  }
  return response.json() as Promise<T>
}

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
