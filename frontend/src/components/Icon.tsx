import type { ReactNode } from 'react'

// Icon — a minimal, consistent 16px stroke icon set (Feather-style paths).
// Staff note: emoji glyphs render differently per platform and break visual
// rhythm; an inline SVG set keeps one visual language, inherits currentColor,
// and adds zero dependencies.

export type IconName =
  | 'search'
  | 'sun'
  | 'moon'
  | 'bell'
  | 'star'
  | 'fork'
  | 'eye'
  | 'code'
  | 'commit'
  | 'issue-open'
  | 'issue-closed'
  | 'pull-request'
  | 'merged'
  | 'file'
  | 'folder'
  | 'branch'
  | 'tag'
  | 'link'
  | 'shield'
  | 'check'
  | 'x'
  | 'clock'
  | 'robot'
  | 'zap'
  | 'key'
  | 'logout'

type Props = {
  name: IconName
  size?: number
  className?: string
}

const PATHS: Record<IconName, ReactNode> = {
  search: (
    <>
      <circle cx="7" cy="7" r="4.5" />
      <line x1="10.5" y1="10.5" x2="14" y2="14" />
    </>
  ),
  sun: (
    <>
      <circle cx="8" cy="8" r="3" />
      <line x1="8" y1="1.5" x2="8" y2="3" /><line x1="8" y1="13" x2="8" y2="14.5" />
      <line x1="1.5" y1="8" x2="3" y2="8" /><line x1="13" y1="8" x2="14.5" y2="8" />
      <line x1="3.4" y1="3.4" x2="4.5" y2="4.5" /><line x1="11.5" y1="11.5" x2="12.6" y2="12.6" />
      <line x1="3.4" y1="12.6" x2="4.5" y2="11.5" /><line x1="11.5" y1="4.5" x2="12.6" y2="3.4" />
    </>
  ),
  moon: <path d="M13.5 9.5A6 6 0 0 1 6.5 2.5a6 6 0 1 0 7 7z" />,
  bell: (
    <>
      <path d="M6 2a4 4 0 0 1 4 4v2.5l1.5 2.5h-11L2 8.5V6a4 4 0 0 1 4-4z" />
      <path d="M5 13a1.5 1.5 0 0 0 3 0" />
    </>
  ),
  star: <path d="M8 1.5l2 4.1 4.5.65-3.25 3.17.77 4.48L8 11.77l-4.02 2.13.77-4.48L1.5 6.25 6 5.6z" />,
  fork: (
    <>
      <circle cx="4.5" cy="3.5" r="1.75" /><circle cx="11.5" cy="3.5" r="1.75" /><circle cx="8" cy="12.5" r="1.75" />
      <path d="M4.5 5.25v1.25a2 2 0 0 0 2 2h3a2 2 0 0 0 2-2V5.25M8 8.5v2.25" />
    </>
  ),
  eye: (
    <>
      <path d="M1.5 8s2.5-4.5 6.5-4.5S14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z" />
      <circle cx="8" cy="8" r="2" />
    </>
  ),
  code: <><path d="M5.5 4.5L2 8l3.5 3.5" /><path d="M10.5 4.5L14 8l-3.5 3.5" /></>,
  commit: (
    <>
      <circle cx="8" cy="8" r="2.75" />
      <line x1="1.5" y1="8" x2="5.25" y2="8" /><line x1="10.75" y1="8" x2="14.5" y2="8" />
    </>
  ),
  'issue-open': (
    <>
      <circle cx="8" cy="8" r="6" /><circle cx="8" cy="8" r="1.5" fill="currentColor" stroke="none" />
    </>
  ),
  'issue-closed': (
    <>
      <circle cx="8" cy="8" r="6" /><path d="M5.5 8l2 2 3-3.5" />
    </>
  ),
  'pull-request': (
    <>
      <circle cx="4" cy="3.5" r="1.75" /><circle cx="4" cy="12.5" r="1.75" /><circle cx="12" cy="12.5" r="1.75" />
      <path d="M4 5.25v5.5M12 10.75V7a2 2 0 0 0-2-2H7.5M9 3.25L7.5 5 9 6.75" />
    </>
  ),
  merged: (
    <>
      <circle cx="4" cy="3.5" r="1.75" /><circle cx="4" cy="12.5" r="1.75" /><circle cx="12" cy="12.5" r="1.75" />
      <path d="M4 5.25v5.5M12 10.75V8a4 4 0 0 0-4-4H5.75M7.5 2.25L5.75 4 7.5 5.75" />
    </>
  ),
  file: <><path d="M9 1.5H4a1 1 0 0 0-1 1v11a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V5.5z" /><path d="M9 1.5V5.5h4" /></>,
  folder: <path d="M1.5 3.5A1 1 0 0 1 2.5 2.5h3.6l1.5 2h5.9a1 1 0 0 1 1 1v7.5a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1z" />,
  branch: (
    <>
      <circle cx="4" cy="3.5" r="1.75" /><circle cx="4" cy="12.5" r="1.75" /><circle cx="12" cy="3.5" r="1.75" />
      <path d="M4 5.25v5.5M12 5.25A4 4 0 0 1 8 9.25H5.75" />
    </>
  ),
  tag: (
    <>
      <path d="M1.5 7V2.5A1 1 0 0 1 2.5 1.5H7l7 7-5.5 5.5z" /><circle cx="5" cy="5" r="1" fill="currentColor" stroke="none" />
    </>
  ),
  link: (
    <>
      <path d="M6.5 9.5l3-3" /><path d="M8.5 4.5l1-1a2.5 2.5 0 0 1 3.54 3.54l-1 1" /><path d="M7.5 11.5l-1 1a2.5 2.5 0 0 1-3.54-3.54l1-1" />
    </>
  ),
  shield: <path d="M8 1.5l5.5 2v4c0 3.5-2.4 5.9-5.5 7-3.1-1.1-5.5-3.5-5.5-7v-4z" />,
  check: <path d="M3 8.5l3.5 3.5L13 5" />,
  x: <><line x1="3.5" y1="3.5" x2="12.5" y2="12.5" /><line x1="12.5" y1="3.5" x2="3.5" y2="12.5" /></>,
  clock: (
    <>
      <circle cx="8" cy="8" r="6" /><path d="M8 4.5V8l2.5 1.5" />
    </>
  ),
  robot: (
    <>
      <rect x="2.5" y="5" width="11" height="7.5" rx="1.5" />
      <circle cx="5.75" cy="8.5" r="0.9" fill="currentColor" stroke="none" /><circle cx="10.25" cy="8.5" r="0.9" fill="currentColor" stroke="none" />
      <line x1="8" y1="2.5" x2="8" y2="5" /><circle cx="8" cy="2" r="0.75" />
    </>
  ),
  zap: <path d="M8.5 1.5L3.5 9h3.5l-.5 5.5L11.5 7H8z" />,
  key: (
    <>
      <circle cx="5" cy="10.5" r="3" /><path d="M7.5 8.5L13.5 2.5M11 5l2 2M9.5 6.5l2 2" />
    </>
  ),
  logout: (
    <>
      <path d="M6 2.5H3.5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1H6" /><path d="M10 5l3 3-3 3M13 8H6.5" />
    </>
  ),
}

export function Icon({ name, size = 16, className }: Props) {
  return (
    <svg
      className={className ? `icon ${className}` : 'icon'}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {PATHS[name]}
    </svg>
  )
}
