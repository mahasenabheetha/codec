// Compare and query state. There is one Compare tab and one Query tab;
// opening either with new inputs replaces what they show.

import { isAbort } from '../../lib/api/client'
import {
  compare,
  getIgnore,
  runQuery,
  saveIgnore,
  type CompareResult,
  type QueryHit,
  type QueryScope,
  type Side,
} from '../../lib/api/compare'
import { layout } from '../../lib/stores/layout.svelte'
import { compareRoute, queryRoute } from '../../lib/stores/router.svelte'
import { openSessions } from '../editor/active.svelte'

/** A side with its what-if buffer filled in, if the file has one. */
function withBuffer(s: Side, useWhatIf: boolean): Side {
  if (s.kind !== 'file' || !s.path || !useWhatIf) return { ...s, content: undefined }
  const session = openSessions.get(s.path)
  return session?.dirty ? { ...s, content: session.buffer } : { ...s, content: undefined }
}

class Compare {
  left = $state<Side>({ kind: 'file' })
  right = $state<Side>({ kind: 'file' })
  // Use open tabs' what-if edits for each side (off = the file on disk).
  leftWhatIf = $state(false)
  rightWhatIf = $state(true)

  result = $state.raw<CompareResult | null>(null)
  error = $state<string | null>(null)
  running = $state(false)
  selected = $state(-1) // index into result.changes

  ignore = $state<string[]>([])
  private ignoreLoaded = false
  private inflight: AbortController | null = null

  ready = $derived(sideReady(this.left) && sideReady(this.right))

  /** Open the Compare tab with these sides. */
  open(left: Side, right: Side, whatIf: { left?: boolean; right?: boolean } = {}) {
    this.left = left
    this.right = right
    this.leftWhatIf = whatIf.left ?? false
    this.rightWhatIf = whatIf.right ?? true
    this.result = null
    layout.open(compareRoute)
    this.run()
  }

  swap() {
    ;[this.left, this.right] = [this.right, this.left]
    ;[this.leftWhatIf, this.rightWhatIf] = [this.rightWhatIf, this.leftWhatIf]
    this.run()
  }

  async loadIgnore() {
    if (this.ignoreLoaded) return
    this.ignoreLoaded = true
    this.ignore = await getIgnore().catch(() => [])
  }

  async setIgnore(patterns: string[]) {
    this.ignore = await saveIgnore(patterns).catch(() => patterns)
    this.run()
  }

  addIgnore(pattern: string) {
    if (!this.ignore.includes(pattern)) this.setIgnore([...this.ignore, pattern])
  }

  async run() {
    if (!this.ready) return
    await this.loadIgnore()
    this.inflight?.abort()
    const ctrl = new AbortController()
    this.inflight = ctrl
    this.running = true
    try {
      const r = await compare(withBuffer(this.left, this.leftWhatIf), withBuffer(this.right, this.rightWhatIf), this.ignore, ctrl.signal)
      if (ctrl.signal.aborted) return
      this.result = r
      this.error = null
      this.selected = -1
    } catch (e) {
      if (!isAbort(e)) this.error = e instanceof Error ? e.message : String(e)
    } finally {
      if (this.inflight === ctrl) this.running = false
    }
  }
}

function sideReady(s: Side): boolean {
  return s.kind === 'helm' ? s.chart !== undefined : !!s.path
}

export const comparison = new Compare()

class Query {
  expr = $state('.spec.template.spec.containers[].image')
  scope = $state<QueryScope>('workspace')
  path = $state('') // file scope
  chart = $state('') // helm scope
  profile = $state('')

  results = $state.raw<QueryHit[] | null>(null)
  truncated = $state(false)
  timedOut = $state(false)
  error = $state<string | null>(null)
  running = $state(false)
  private inflight: AbortController | null = null

  /** Open the Query tab, optionally aimed at one file or chart. */
  open(target?: { path?: string; chart?: string }) {
    if (target?.path) {
      this.scope = 'file'
      this.path = target.path
    } else if (target?.chart !== undefined) {
      this.scope = 'helm'
      this.chart = target.chart
    }
    layout.open(queryRoute)
  }

  async run() {
    this.inflight?.abort()
    const ctrl = new AbortController()
    this.inflight = ctrl
    this.running = true
    const side: Partial<Side> =
      this.scope === 'helm'
        ? { kind: 'helm', chart: this.chart, profile: this.profile }
        : this.scope === 'file'
          ? withBuffer({ kind: 'file', path: this.path }, true)
          : {}
    try {
      const r = await runQuery(this.expr, this.scope, side, ctrl.signal)
      if (ctrl.signal.aborted) return
      this.results = r.results
      this.truncated = r.truncated
      this.timedOut = r.timedOut
      this.error = null
    } catch (e) {
      if (!isAbort(e)) {
        this.error = e instanceof Error ? e.message : String(e)
        this.results = null
      }
    } finally {
      if (this.inflight === ctrl) this.running = false
    }
  }
}

export const queries = new Query()
