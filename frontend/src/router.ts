import { useEffect, useState } from 'react'

// Minimal hash router: "#/alice/castle/tree/main/src" style paths.
// Chosen over history API so the built app works from any static host
// without rewrite rules.

export type Route =
  | { page: 'home' }
  | { page: 'login' }
  | { page: 'repo'; owner: string; name: string; tab: 'code' | 'commits'; rev: string; filePath: string; commitHash?: string }

function parse(hash: string): Route {
  const clean = hash.replace(/^#\/?/, '')
  const parts = clean.split('/').filter(Boolean)
  if (parts[0] === 'login') return { page: 'login' }
  if (parts.length >= 2) {
    const [owner, name, third, fourth, ...rest] = parts
    const filePath = rest.join('/')
    if (third === 'commits') {
      return { page: 'repo', owner, name, tab: 'commits', rev: fourth ?? 'HEAD', filePath: '' }
    }
    if (third === 'commit' && fourth) {
      return { page: 'repo', owner, name, tab: 'commits', rev: 'HEAD', filePath: '', commitHash: fourth }
    }
    // code tab: #/o/n/tree/{rev}/{...path} or #/o/n/blob/{rev}/{...path}
    const kind = (third === 'blob' ? 'blob' : 'tree') as 'tree' | 'blob'
    return { page: 'repo', owner, name, tab: 'code', rev: fourth ?? 'HEAD', filePath, ...(kind === 'blob' ? {} : {}) }
  }
  return { page: 'home' }
}

export function useRoute(): Route {
  const [route, setRoute] = useState<Route>(() => parse(window.location.hash))
  useEffect(() => {
    const onChange = () => setRoute(parse(window.location.hash))
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])
  return route
}

export function navigate(to: string): void {
  window.location.hash = to.startsWith('#') ? to : `#${to}`
}
