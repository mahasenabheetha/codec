import { request } from './client'
import type { Range } from './yaml'

/** Something to compare or query: a workspace file (its what-if buffer
 *  when content is set) or a chart rendered with a saved profile. */
export interface Side {
  kind: 'file' | 'helm' | 'paste'
  path?: string
  content?: string // file: what-if buffer; paste: the text (never stored)
  doc?: number // file, paste: only this document (0-based)
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
  rows?: DiffRow[] // text mode: side by side, unchanged runs folded
  rowsTruncated?: boolean
  note?: string
}

/** One line of a side-by-side text diff. A gap folds `count` unchanged
 *  lines starting at left.line / right.line. */
export interface DiffRow {
  kind: 'equal' | 'change' | 'delete' | 'insert' | 'gap'
  left?: DiffCell
  right?: DiffCell
  count?: number
}

export interface DiffCell {
  line: number // 1-based
  text: string
  spans?: [number, number][] // changed parts, UTF-16 offsets
}

export type CompareMode = 'auto' | 'structure' | 'text'

export interface CompareOptions {
  mode?: CompareMode
  ignoreSpace?: boolean // text diffs only
  ignoreCase?: boolean
}

export const compare = (left: Side, right: Side, ignore: string[], opts: CompareOptions = {}, signal?: AbortSignal) =>
  request<CompareResult>('POST', '/api/v2/compare', { left, right, ignore, ...opts }, signal)

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
