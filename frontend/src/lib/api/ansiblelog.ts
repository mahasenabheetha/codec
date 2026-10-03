import { postRaw } from './client'

// A whole Ansible log read by the server (internal/ansiblelog): runs,
// plays, tasks and host results, and the output around them. Line
// numbers are the input's, 1-based.

export type LogStatus = 'ok' | 'changed' | 'skipping' | 'failed' | 'unreachable' | 'included' | 'rescued' | 'unfinished'

export interface LogAnalysis {
  lines: number
  blocks: LogBlock[]
  summary: LogSummary
}

export interface LogSummary {
  runs: number
  hosts: string[]
  tasks: number
  ok: number
  changed: number
  failed: number
  unreachable: number
  skipped: number
  rescued: number
  ignored: number
  durationMs?: number
  other: number
  otherErrors: number
  firstFailure?: { Block: number; Play: number; Task: number; Result: number }
  verdict?: string
}

export interface LogBlock {
  kind: 'run' | 'other'
  from: number
  to: number
  title?: string
  run?: LogRun
  text?: string[]
  truncated?: boolean
  errors?: number
  warnings?: number
}

export interface LogRun {
  playbook?: string
  json?: boolean
  complete: boolean
  status: LogStatus
  durationMs?: number
  hosts: string[]
  plays: LogPlay[]
  recap: LogRecap[]
  notes: { line: number; level: 'error' | 'warning' | 'deprecation'; text: string }[]
}

export interface LogRecap {
  host: string
  ok: number
  changed: number
  unreachable: number
  failed: number
  skipped: number
  rescued: number
  ignored: number
}

export interface LogPlay {
  name: string
  line?: number
  tasks: LogTask[]
}

export interface LogTask {
  name: string
  role?: string
  handler?: boolean
  line?: number
  path?: string
  pathLine?: number
  durationMs?: number
  status: LogStatus
  counts: Partial<Record<'ok' | 'changed' | 'failed' | 'unreachable' | 'skipped' | 'ignored' | 'rescued' | 'included', number>>
  results: LogResult[]
  other?: string[]
}

export interface LogResult {
  host: string
  status: LogStatus
  line?: number
  endLine?: number
  delegate?: string
  item?: string
  file?: string
  retries?: number
  ignored?: boolean
  rescued?: boolean
  censored?: boolean
  msg?: string
  format?: 'json' | 'yaml' | 'raw'
  payload?: Record<string, unknown>
  raw?: string
}

/** Read a log: a dropped file goes up as it is, up to 64 MB. */
export const analyzeLog = (body: Blob | string, signal?: AbortSignal) => postRaw<LogAnalysis>('/api/v2/ansible/log', body, signal)
