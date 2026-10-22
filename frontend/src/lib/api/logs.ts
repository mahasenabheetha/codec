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
  message: string
  more?: boolean
  format: string
  error?: string
  stack?: boolean
  cols?: Record<string, string>
}

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
  total: number
  levels: number[]
  warnings?: string[]
  offset: number
  entries: LogRow[]
  histogram?: Histogram
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
  text: string
  level: Level
  count: number
  first: number
  last: number
  from?: string
  to?: string
  sources: number
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
  }
  return postForm('/api/v2/logs', form, signal)
}

const base = (id: string) => `/api/v2/logs/${encodeURIComponent(id)}`

export function logEntries(
  id: string,
  req: LogQuery & { offset?: number; limit?: number; columns?: string[]; histogram?: number; at?: number },
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

export function closeLog(id: string): Promise<void> {
  return request('DELETE', base(id))
}
