import { useEffect, useState } from 'react'

// Minimal hash router: "#/alice/castle/tree/main/src" style paths.
// Chosen over history API so the built app works from any static host
// without rewrite rules.

export type Route =
  | { page: 'home' }
  | { page: 'login' }
  | { page: 'repo'; owner: string; name: string; tab: 'code' | 'commits' | 'issues' | 'pulls'; rev: string; filePath: string; commitHash?: string; subjectNumber?: number }

function parse(hash: string): Route {
  const clean = hash.replace(/^#\/?/, '')
  const parts = clean.split('/').filter(Boolean)
  if (parts[0] === 'login') return { page: 'login' }
  if (parts.length >= 2) {
    const [owner, name, third, fourth, ...rest] = parts
    const filePath = rest.join('/')
    if (third === 'issues') {
      return { page: 'repo', owner, name, tab: 'issues', rev: 'HEAD', filePath: '', subjectNumber: fourth ? Number(fourth) : undefined }
    }
    if (third === 'pulls') {
      return { page: 'repo', owner, name, tab: 'pulls', rev: 'HEAD', filePath: '', subjectNumber: fourth ? Number(fourth) : undefined }
    }
    if (third === 'commits') {
      return { page: 'repo', owner, name, tab: 'commits', rev: fourth ?? 'HEAD', filePath: '' }
    }
    if (third === 'commit' && fourth) {
      return { page: 'repo', owner, name, tab: 'commits', rev: 'HEAD', filePath: '', commitHash: fourth }
    }
    // code tab: #/o/n/tree/{rev}/{...path}
    return { page: 'repo', owner, name, tab: 'code', rev: fourth ?? 'HEAD', filePath }
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
