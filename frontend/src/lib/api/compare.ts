import { request } from './client'
import type { Range } from './yaml'

/** Something to compare or query: a workspace file (its what-if buffer
 *  when content is set) or a chart rendered with a saved profile. */
export interface Side {
  kind: 'file' | 'helm'
  path?: string
  content?: string
  doc?: number // file: only this document (0-based)
  chart?: string
  profile?: string // "" = the chart's defaults
}

export interface Change {
  kind: 'added' | 'removed' | 'changed' | 'reordered'
  doc: string // "Deployment prod/api" or "#2"
  path: string // "" = the whole document
  old?: string
  new?: string
  oldRange?: Range
  newRange?: Range
}

export interface CompareResult {
  mode: 'semantic' | 'text'
  changes: Change[]
  left: { title: string; text: string }
  right: { title: string; text: string }
  diff?: string // text mode: unified diff
  note?: string
}

export const compare = (left: Side, right: Side, ignore: string[], signal?: AbortSignal) =>
  request<CompareResult>('POST', '/api/v2/compare', { left, right, ignore }, signal)

export const getIgnore = () => request<{ patterns: string[] }>('GET', '/api/v2/compare/ignore').then((r) => r.patterns)

export const saveIgnore = (patterns: string[]) =>
  request<{ patterns: string[] }>('POST', '/api/v2/compare/ignore', { patterns }).then((r) => r.patterns)

export interface QueryHit {
  file?: string // workspace path; absent for a Helm render
  doc: number
  path?: string // jq path; absent for computed values
  value: string // YAML
  range?: Range
}

export type QueryScope = 'workspace' | 'file' | 'helm'

export const runQuery = (expr: string, scope: QueryScope, side: Partial<Side>, signal?: AbortSignal) =>
  request<{ results: QueryHit[]; truncated: boolean; timedOut: boolean }>('POST', '/api/v2/query', { expr, scope, ...side }, signal)
