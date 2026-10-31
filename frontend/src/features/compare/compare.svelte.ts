// Compare and query state. Compare has work tabs (decision 97), each a
// comparison of its own; the ignored paths (from the settings) and the
// Explorer's picked file are shared. There is one Query tab.

import { isAbort } from '../../lib/api/client'
import {
  compare,
  getIgnore,
  runQuery,
  saveIgnore,
  type CompareMode,
  type CompareResult,
  type QueryHit,
  type QueryScope,
  type Side,
} from '../../lib/api/compare'
import { layout } from '../../lib/stores/layout.svelte'
import { compareRoute, queryRoute } from '../../lib/stores/router.svelte'
import { toasts } from '../../lib/stores/toast.svelte'
import { openSessions } from '../editor/active.svelte'
import { newTabKey, WorkTabs } from '../shared/worktabs.svelte'

/** A side with its what-if buffer filled in, if the file has one. */
function withBuffer(s: Side, useWhatIf: boolean): Side {
  if (s.kind === 'paste') return s
  if (s.kind !== 'file' || !s.path || !useWhatIf) return { ...s, content: undefined }
  const session = openSessions.get(s.path)
  return session?.dirty ? { ...s, content: session.buffer } : { ...s, content: undefined }
}

type Which = 'left' | 'right'

/** The ignored paths: one list, saved in the settings, for every tab. */
class Ignores {
  list = $state<string[]>([])
  #loaded = false

  async load() {
    if (this.#loaded) return
    this.#loaded = true
    this.list = await getIgnore().catch(() => [])
  }

  async set(patterns: string[]) {
    this.list = await saveIgnore(patterns).catch(() => patterns)
    for (const c of compareTabs.list) c.run()
  }
}

const ignores = new Ignores()

/** One comparison: a Compare tab's sides, options and result. */
export class Comparison {
  readonly key = newTabKey()
  /** Its number among the open tabs (set by the store). */
  n = $state(0)
  /** The user's own name; '' names it after its sides. */
  name = $state('')
  left = $state<Side>({ kind: 'file' })
  right = $state<Side>({ kind: 'file' })
  // Use open tabs' what-if edits for each side (off = the file on disk).
  leftWhatIf = $state(false)
  rightWhatIf = $state(true)

  result = $state.raw<CompareResult | null>(null)
  error = $state<string | null>(null)
  running = $state(false)
  selected = $state(-1) // index into result.changes

  // How to compare, and what a text diff may overlook.
  mode = $state<CompareMode>('auto')
  ignoreSpace = $state(false)
  ignoreCase = $state(false)
  // How a text diff is shown.
  textView = $state<'split' | 'unified'>('split')

  private inflight: AbortController | null = null
  private typing: ReturnType<typeof setTimeout> | undefined

  ready = $derived(sideReady(this.left) && sideReady(this.right))
  empty = $derived(!sideChosen(this.left) && !sideChosen(this.right))

  /** The tab's name: the user's, or its two sides. */
  get title(): string {
    if (this.name) return this.name
    if (this.empty) return `Compare ${this.n}`
    // Two pasted sides have no names to show; number them instead.
    if (this.left.kind === 'paste' && this.right.kind === 'paste') return `Pasted ${this.n}`
    return `${sideName(this.left)} ↔ ${sideName(this.right)}`
  }

  /** Something pasted on either side (it exists nowhere else, 19). */
  get hasPaste(): boolean {
    return pasted(this.left) || pasted(this.right)
  }

  /** Show these sides. */
  set(left: Side, right: Side, whatIf: { left?: boolean; right?: boolean } = {}) {
    this.left = left
    this.right = right
    this.leftWhatIf = whatIf.left ?? false
    this.rightWhatIf = whatIf.right ?? true
    this.result = null
    this.run()
  }

  /** A paste box on each side; a side already pasted keeps its text. */
  pasteBoth() {
    for (const w of ['left', 'right'] as const) {
      if (this[w].kind !== 'paste') this[w] = { kind: 'paste', content: '' }
    }
    this.result = null
    this.error = null
    this.run()
  }

  /** Put something else on one side. Pasted text it replaces exists
   *  nowhere else (decision 19), so that can be undone. */
  setSide(which: Which, next: Side) {
    const before = this[which]
    const whatIf = this[`${which}WhatIf` as const]
    this[which] = next
    if (pasted(before)) this.offerUndo([[which, before, whatIf]])
  }

  /** A "Cleared · Undo" toast that puts back the given sides, each only
   *  if nothing has replaced it since. */
  private offerUndo(lost: [Which, Side, boolean][]) {
    const after = lost.map(([w]) => this[w])
    toasts.show('Cleared', 'neutral', 8000, {
      label: 'Undo',
      run: () => {
        let restored = false
        lost.forEach(([w, side, whatIf], i) => {
          if (this[w] !== after[i]) return
          this[w] = side
          this[`${w}WhatIf` as const] = whatIf
          restored = true
        })
        if (restored) this.run()
      },
    })
  }

  /** Run after typing in a paste box settles. */
  runSoon() {
    clearTimeout(this.typing)
    this.typing = setTimeout(() => this.run(), 400)
  }

  swap() {
    ;[this.left, this.right] = [this.right, this.left]
    ;[this.leftWhatIf, this.rightWhatIf] = [this.rightWhatIf, this.leftWhatIf]
    this.run()
  }

  /** Empty one side, or both; the ignore patterns stay. */
  clear(which: 'left' | 'right' | 'both' = 'both') {
    // Pasted text exists nowhere else (decision 19), so offer it back.
    const lost: [Which, Side, boolean][] = []
    if (which !== 'right' && pasted(this.left)) lost.push(['left', this.left, this.leftWhatIf])
    if (which !== 'left' && pasted(this.right)) lost.push(['right', this.right, this.rightWhatIf])
    clearTimeout(this.typing)
    this.inflight?.abort()
    this.running = false
    if (which !== 'right') {
      this.left = { kind: 'file' }
      this.leftWhatIf = false
    }
    if (which !== 'left') {
      this.right = { kind: 'file' }
      this.rightWhatIf = true
    }
    this.result = null
    this.error = null
    this.selected = -1
    if (lost.length) this.offerUndo(lost)
  }

  /** The ignored paths, shared by every tab. */
  get ignore(): string[] {
    return ignores.list
  }

  setIgnore(patterns: string[]) {
    return ignores.set(patterns)
  }

  addIgnore(pattern: string) {
    if (!ignores.list.includes(pattern)) ignores.set([...ignores.list, pattern])
  }

  async run() {
    if (!this.ready) return
    this.inflight?.abort()
    const ctrl = new AbortController()
    this.inflight = ctrl
    this.running = true
    try {
      // A clear() while the patterns load aborts this run.
      await ignores.load()
      if (ctrl.signal.aborted || !this.ready) return
      const r = await compare(withBuffer(this.left, this.leftWhatIf), withBuffer(this.right, this.rightWhatIf), this.ignore, { mode: this.mode, ignoreSpace: this.ignoreSpace, ignoreCase: this.ignoreCase }, ctrl.signal)
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
  if (s.kind === 'paste') return pasted(s)
  return s.kind === 'helm' ? s.chart !== undefined : !!s.path
}

function pasted(s: Side): boolean {
  return s.kind === 'paste' && !!s.content?.trim()
}

/** A side has something chosen (a Helm or paste side counts even when empty). */
export function sideChosen(s: Side): boolean {
  return s.kind !== 'file' || !!s.path
}

/** A side in a tab's name: a file's name, a chart's render, or pasted. */
function sideName(s: Side): string {
  if (s.kind === 'paste') return 'pasted'
  if (s.kind === 'helm') return `${(s.chart || 'chart').split('/').pop()}${s.profile ? ` (${s.profile})` : ''}`
  return s.path ? s.path.split('/').pop()! : '…'
}

/** Compare's work tabs, and what the rest of codec asks of Compare:
 *  opening it on files, renders or pasted text (in the current tab when
 *  that is empty, else in a new one), the shared ignored paths, and the
 *  Explorer's picked file. */
class CompareTabs extends WorkTabs<Comparison> {
  // The file chosen with "Select for compare" in the Explorer; "Compare
  // with selected" pairs another file with it.
  picked = $state<string | null>(null)

  constructor() {
    super(() => new Comparison(), { keepOne: true, undoable: (c) => c.hasPaste })
  }

  /** The current tab when it has nothing yet, else a new one. */
  #free(): Comparison {
    const cur = this.current
    return cur && cur.empty ? cur : this.add()
  }

  /** Open Compare on these sides. */
  open(left: Side, right: Side, whatIf: { left?: boolean; right?: boolean } = {}) {
    this.#free().set(left, right, whatIf)
    layout.open(compareRoute)
  }

  /** Open Compare on two workspace files. */
  openFiles(left: string, right: string) {
    this.open({ kind: 'file', path: left }, { kind: 'file', path: right })
  }

  /** Open Compare with a paste box on each side: the current tab if it is
   *  empty or already pasting, else a new one. */
  pasteBoth() {
    const cur = this.current
    const tab = cur && (cur.empty || cur.left.kind === 'paste' || cur.right.kind === 'paste') ? cur : this.add()
    tab.pasteBoth()
    layout.open(compareRoute)
  }

  /** Compare the current tab again. */
  run() {
    this.current?.run()
  }

  get ignore(): string[] {
    return ignores.list
  }
  loadIgnore() {
    return ignores.load()
  }
  setIgnore(patterns: string[]) {
    return ignores.set(patterns)
  }
  addIgnore(pattern: string) {
    if (!ignores.list.includes(pattern)) ignores.set([...ignores.list, pattern])
  }
}

export const compareTabs = new CompareTabs()
/** Compare as the rest of codec uses it. */
export const comparison = compareTabs

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
