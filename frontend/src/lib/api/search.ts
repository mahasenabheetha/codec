import { request } from './client'

export interface SearchOptions {
  query: string
  regex?: boolean
  matchCase?: boolean
  wholeWord?: boolean
  globs?: string[] // "charts/**"; "!" excludes
}

export interface SearchMatch {
  path: string
  line: number // 1-based
  text: string // the line, maybe cut to a window ("…" at a cut end)
  spans: { start: number; end: number }[] // hits in text
  col: number // first hit in the full line, 1-based UTF-16
  endCol: number // its end, exclusive
}

export interface SearchResult {
  matches: SearchMatch[]
  files: number
  searched: number
  truncated: boolean
  timedOut: boolean
}

export const searchFiles = (opts: SearchOptions, signal?: AbortSignal) =>
  request<SearchResult>('POST', '/api/v2/search', opts, signal)
