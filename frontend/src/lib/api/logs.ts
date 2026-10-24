import { postForm, request, requestBlob } from './client'

// The Log viewer's server side (internal/logview, decision 86): a log
// read once and held in memory under an id; the UI asks for pages,
// counts and charts. An id the server no longer holds fails with 410,
// and the viewer sends its sources again.

export type Level = '' | 'trace' | 'debug' | 'info' | 'warn' | 'error' | 'fatal'
/** Histogram buckets count levels in this order. */
export const levels: Level[] = ['', 'trace', 'debug', 'info', 'warn', 'error', 'fatal']

export interface LogFileInfo {
  name: string
  bytes: number
  lines: number
  entries: number
  encoding?: string[]
}

export interface LogSummary {
  entries: number
  lines: number
  formats: Record<string, number>
  levels: Record<string, number> // '' counts entries without a level
  timed: number
  from?: string
  to?: string
  sources?: { value: string; count: number }[]
  firstError: number // entry index, -1 when none
  stacks: number
  groups: number
  gaps?: { after: number; from: string; to: string; seconds: number }[]
  /** An Argo run: its workflows and steps, in the order the steps start. */
  workflows?: string[]
  steps?: LogStep[]
}

export interface LogStep {
  name: string
  pods: number
  entries: number
  errors: number
  warnings: number
  from?: string
  to?: string
  first: number // its first entry
}

export interface LogOpened {
  id: string
  files: LogFileInfo[]
  summary: LogSummary
  size: number
}

export interface LogRow {
  i: number
  file: number
  line: number
  lines?: number
  time?: string
  level?: Level
  source?: string
  step?: string // an Argo run's step
  message: string
  more?: boolean
  format: string
  error?: string
  stack?: boolean
  cols?: Record<string, string>
  /** In a view: '' for a match, else context, gap, output or repeat. */
  kind?: '' | 'context' | 'gap' | 'output' | 'repeat'
  n?: number // entries a fold holds or a gap skips
  matches?: number // of those, the ones the query matched
  lastI?: number
  lastTime?: string
  hl?: number // highlight rules matched, a bit each
}

/** How the matches are laid out (internal/logview/view.go). */
export interface ViewOptions {
  context?: number // entries around each match
  output?: boolean // fold runs of plain output
  repeats?: '' | 'exact' | 'similar'
  expanded?: number[] // folds shown whole, by their first entry
}

/** Marker bits per bucket, for beside the scrollbar; rule k is markRule << k. */
export const markError = 1
export const markWarn = 2
export const markPin = 4
export const markRule = 8

export interface Histogram {
  from?: string
  stepMs: number
  buckets: number[][] // per bucket, counts by level (see levels)
  untimed: number
}

export interface LogQuery {
  q: string
  from?: string
  to?: string
}

export interface EntriesPage {
  total: number // matches
  levels: number[]
  warnings?: string[]
  rows: number // in the view: matches, context, gaps and folds
  offset: number // of the page, in rows
  atRow?: number // the row holding the entry asked for with `at`; -1 when the view doesn't show it
  entries: LogRow[]
  histogram?: Histogram
  markers?: number[]
  /** Per marker, the row its mark stands for; -1 where there is none. */
  markerRows?: number[]
  highlightErrors?: string[]
}

export interface EntryField {
  key: string
  kind: 'string' | 'number' | 'bool' | 'null' | 'array' | 'object'
  text: string
}

export interface LogEntry {
  i: number
  file: string
  line: number
  lines: number
  format: string
  source?: string
  step?: string
  time?: string
  level?: Level
  rawLevel?: string
  message: string
  logger?: string
  caller?: string
  error?: string
  stack?: string
  traceId?: string
  fields: EntryField[]
  raw: string
  rawCut?: boolean
}

export interface FieldStat {
  key: string
  count: number
  distinct: number
  more?: boolean
  long?: boolean
  top?: { value: string; count: number }[]
  num?: { min: number; max: number; mean: number; median: number; p95: number }
}

export interface LogGroup {
  id: number // for the query group:ID
  text: string
  level: Level
  count: number
  first: number
  last: number
  from?: string
  to?: string
  sources: number
}

/** Two runs compared (internal/logview/compare.go): pairs hold this
 *  run's value first, the other's second; -1 where there is no entry. */
export interface LogRunStats {
  entries: number
  errors: number
  warnings: number
  seconds: number
  messages: number
}

export interface LogDiffItem {
  text: string // the fingerprint, as matched
  sample: string
  level: Level
  count: [number, number]
  first: [number, number]
}

export interface LogSourceDiff {
  name: [string, string]
  entries: [number, number]
  errors: [number, number]
  seconds: [number, number]
  first: [number, number]
}

export interface LogDiff {
  runs: [LogRunStats, LogRunStats]
  first: [number, number]
  firstRows: [LogRow | null, LogRow | null]
  new: LogDiffItem[] | null
  missing: LogDiffItem[] | null
  changed: LogDiffItem[] | null
  more: [number, number, number]
  sources?: LogSourceDiff[]
}

/** A log to read: a file (dropped or picked), pasted text, or a file in
 *  the open folder. */
export type LogSource = { kind: 'file'; file: File } | { kind: 'text'; name: string; text: string } | { kind: 'path'; path: string }

export function openLogs(sources: LogSource[], signal?: AbortSignal): Promise<LogOpened> {
  const paths = sources.filter((s) => s.kind === 'path').map((s) => s.path)
  if (paths.length === sources.length) return request('POST', '/api/v2/logs', { paths }, signal)
  const form = new FormData()
  for (const s of sources) {
    if (s.kind === 'file') form.append('file', s.file, s.file.name)
    else if (s.kind === 'text') form.append('file', new Blob([s.text], { type: 'text/plain' }), s.name)
    else form.append('path', s.path) // a file in the open folder, beside dropped ones
  }
  return postForm('/api/v2/logs', form, signal)
}

const base = (id: string) => `/api/v2/logs/${encodeURIComponent(id)}`

export function logEntries(
  id: string,
  req: LogQuery & {
    view?: ViewOptions
    offset?: number
    limit?: number
    columns?: string[]
    histogram?: number
    at?: number
    highlights?: string[]
    markers?: number
    pins?: number[]
  },
  signal?: AbortSignal,
): Promise<EntriesPage> {
  return request('POST', base(id) + '/entries', req, signal)
}

export function logEntry(id: string, n: number, around = 0, signal?: AbortSignal): Promise<{ entry: LogEntry; around: LogRow[] }> {
  return request('GET', `${base(id)}/entries/${n}?before=${around}&after=${around}`, undefined, signal)
}

export function logFields(id: string, req: LogQuery & { top?: number }, signal?: AbortSignal): Promise<{ total: number; fields: FieldStat[] }> {
  return request('POST', base(id) + '/fields', req, signal)
}

export function logGroups(id: string, req: LogQuery, signal?: AbortSignal): Promise<{ groups: LogGroup[] }> {
  return request('POST', base(id) + '/groups', req, signal)
}

export function exportLog(id: string, req: LogQuery & { format: 'text' | 'ndjson' | 'csv'; columns?: string[] }): Promise<Blob> {
  return requestBlob(base(id) + '/export', req)
}

/** How log id differs from log other; a 410 with body.other set means
 *  the other log is the one gone. */
export function compareLogs(id: string, other: string, signal?: AbortSignal): Promise<LogDiff> {
  return request('POST', base(id) + '/compare', { other }, signal)
}

export function closeLog(id: string): Promise<void> {
  return request('DELETE', base(id))
}
