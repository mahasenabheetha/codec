// The Ansible log analyzer's state: the log being read and what is
// selected in it. Kept in memory only, never stored (decision 19); the
// playbook map reads the loaded runs to colour its nodes.

import { isAbort } from '../../lib/api/client'
import { analyzeLog, type LogAnalysis, type LogResult, type LogStatus, type LogTask } from '../../lib/api/ansiblelog'

export type LogFilter = 'failed' | 'changed' | 'skipped' | 'all'

/** What the detail pane shows; indexes into analysis.blocks. */
export type LogSelection =
  | { kind: 'task'; block: number; play: number; task: number; result: number }
  | { kind: 'run'; block: number }
  | { kind: 'other'; block: number }
  | null

class LogState {
  analysis = $state.raw<LogAnalysis | null>(null)
  source = $state.raw<{ name: string; size: number } | null>(null)
  running = $state(false)
  error = $state<string | null>(null)

  sel = $state.raw<LogSelection>(null)
  filter = $state<LogFilter>('all')
  host = $state('') // '' = every host
  query = $state('')

  private ctrl: AbortController | null = null

  /** Read a log: pasted text, or a dropped or picked file. */
  async analyze(body: Blob | string, name: string) {
    this.ctrl?.abort()
    const c = new AbortController()
    this.ctrl = c
    this.running = true
    this.error = null
    try {
      const a = await analyzeLog(body, c.signal)
      if (c.signal.aborted) return
      this.analysis = a
      this.source = { name, size: typeof body === 'string' ? body.length : body.size }
      this.host = ''
      this.query = ''
      this.filter = 'all'
      this.selectFirst()
    } catch (e) {
      if (!isAbort(e)) this.error = e instanceof Error ? e.message : String(e)
    } finally {
      if (this.ctrl === c) this.running = false
    }
  }

  /** The first failure, else the first run, else nothing. */
  selectFirst() {
    const a = this.analysis
    const f = a?.summary.firstFailure
    if (f) {
      this.sel = { kind: 'task', block: f.Block, play: f.Play, task: f.Task, result: f.Result }
      return
    }
    const run = a?.blocks.findIndex((b) => b.kind === 'run') ?? -1
    this.sel = run >= 0 ? { kind: 'run', block: run } : a?.blocks.length ? { kind: 'other', block: 0 } : null
  }

  clear() {
    this.ctrl?.abort()
    this.analysis = null
    this.source = null
    this.sel = null
    this.error = null
    this.running = false
  }

  /** The runs, with their block index, for pickers. */
  runs = $derived((this.analysis?.blocks ?? []).flatMap((b, i) => (b.run ? [{ block: i, run: b.run, from: b.from }] : [])))
}

export const logs = new LogState()

/** Whether a task passes the outline's filter and host choice. */
export function taskShown(t: LogTask, filter: LogFilter, host: string, query: string): boolean {
  if (host && !t.results.some((r) => r.host === host)) return false
  const q = query.trim().toLowerCase()
  if (q && !`${t.role ?? ''} ${t.name} ${t.results.map((r) => r.msg ?? '').join(' ')}`.toLowerCase().includes(q)) return false
  const c = t.counts
  switch (filter) {
    case 'failed':
      return ['failed', 'unreachable', 'unfinished', 'rescued'].includes(t.status) || !!c.ignored
    case 'changed':
      return t.status === 'changed' || !!c.changed
    case 'skipped':
      return t.status === 'skipping' || !!c.skipped
  }
  return true
}

/** The status a result shows as: ignored and rescued failures apart. */
export function resultStatus(r: LogResult): LogStatus | 'ignored' {
  if (r.ignored) return 'ignored'
  if (r.rescued) return 'rescued'
  return r.status
}

export const statusTone: Record<string, 'ok' | 'warn' | 'err' | 'neutral' | 'accent'> = {
  ok: 'ok',
  changed: 'warn',
  failed: 'err',
  unreachable: 'err',
  unfinished: 'err',
  rescued: 'accent',
  ignored: 'neutral',
  skipping: 'neutral',
  included: 'neutral',
}

/** "850ms", "12.4s", "3m05s", "1h23m". */
export function duration(ms?: number): string {
  if (!ms) return ''
  if (ms < 1000) return `${ms}ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)}s`
  if (s < 3600) return `${Math.floor(s / 60)}m${String(Math.floor(s % 60)).padStart(2, '0')}s`
  return `${Math.floor(s / 3600)}h${String(Math.floor((s % 3600) / 60)).padStart(2, '0')}m`
}

export const taskLabel = (t: LogTask) => (t.role ? `${t.role} : ${t.name}` : t.name)
