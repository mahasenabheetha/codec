// One open file: what's on disk, the editor buffer (what-if edits live
// only here, never saved), and the latest analysis of the buffer.

import { isAbort } from '../../lib/api/client'
import { getContent, type FileContent } from '../../lib/api/workspace'
import { analyze, type Analysis, type YamlSymbol } from '../../lib/api/yaml'

/** CodeMirror works on LF text; the header shows the real endings. */
export function toLF(s: string): string {
  return s.replace(/\r\n?/g, '\n')
}

/** A symbol with where it sits: a stable id and its parent. */
export interface SymbolRef {
  id: string
  sym: YamlSymbol
  parent: SymbolRef | null
}

export class FileSession {
  readonly path: string

  disk = $state.raw<FileContent | null>(null)
  diskText = $derived(this.disk ? toLF(this.disk.text) : '')
  buffer = $state('')
  dirty = $derived(this.disk !== null && this.buffer !== this.diskText)
  /** The disk changed while there were what-if edits: ask first. */
  pendingDisk = $state.raw<FileContent | null>(null)
  error = $state<Error | null>(null)

  analysis = $state.raw<Analysis | null>(null)
  /** The buffer text the analysis describes. */
  analyzedText = ''
  cursor = $state({ line: 1, col: 1 })

  private timer: ReturnType<typeof setTimeout> | undefined
  private inflight: AbortController | null = null

  constructor(path: string) {
    this.path = path
  }

  get isYAML(): boolean {
    return this.disk?.lang === 'yaml'
  }

  /** Read from disk. With edits pending, a disk change waits for the
   *  user (keep or reload) instead of overwriting their work. */
  async load(): Promise<'loaded' | 'updated' | 'conflict' | 'same'> {
    try {
      const c = await getContent(this.path)
      this.error = null
      if (this.disk && c.text === this.disk.text) return 'same'
      if (this.dirty) {
        this.pendingDisk = c
        return 'conflict'
      }
      const first = this.disk === null
      this.disk = c
      this.buffer = toLF(c.text)
      return first ? 'loaded' : 'updated'
    } catch (e) {
      this.error = e instanceof Error ? e : new Error(String(e))
      return 'same'
    }
  }

  /** Take the pending disk version, dropping what-if edits. */
  reloadFromDisk() {
    const c = this.pendingDisk ?? this.disk
    this.pendingDisk = null
    if (!c) return
    this.disk = c
    this.buffer = toLF(c.text)
  }

  /** Keep editing on top of the new disk version. */
  keepEdits() {
    if (this.pendingDisk) this.disk = this.pendingDisk
    this.pendingDisk = null
  }

  reset() {
    this.buffer = this.diskText
  }

  /** Content to send with a request: omitted when the buffer is the
   *  disk version, so the server uses its cached parse. */
  content(): string | undefined {
    return this.dirty ? this.buffer : undefined
  }

  /** Analyze the buffer, debounced; a newer call cancels older ones. */
  scheduleAnalyze(delay = 150) {
    if (!this.isYAML) return
    // Bigger files wait longer for a pause: their results cost more to show.
    delay += Math.min(450, Math.floor(this.buffer.length / 400))
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.runAnalyze(), delay)
  }

  private async runAnalyze() {
    this.inflight?.abort()
    const ctrl = new AbortController()
    this.inflight = ctrl
    const text = this.buffer
    try {
      const a = await analyze(this.path, this.content(), ctrl.signal)
      if (ctrl.signal.aborted) return
      this.analyzedText = text
      this.analysis = a
      // A schema was still downloading: ask again shortly (unless the
      // user types first, which re-analyzes anyway).
      if (a.docs.some((d) => d.schema?.state === 'pending')) {
        clearTimeout(this.timer)
        this.timer = setTimeout(() => this.runAnalyze(), 2000)
      }
    } catch (e) {
      if (!isAbort(e)) this.analysis = null
    }
  }

  dispose() {
    clearTimeout(this.timer)
    this.inflight?.abort()
  }

  // --- derived views of the analysis ---

  /** Every symbol, flattened, with ids like "0/2/1" (doc/index/index). */
  symbols = $derived.by<SymbolRef[]>(() => {
    const out: SymbolRef[] = []
    const walk = (syms: YamlSymbol[], id: string, parent: SymbolRef | null) =>
      syms.forEach((s, i) => {
        const ref: SymbolRef = { id: `${id}/${i}`, sym: s, parent }
        out.push(ref)
        if (s.children) walk(s.children, ref.id, ref)
      })
    for (const d of this.analysis?.docs ?? []) walk(d.symbols, String(d.index), null)
    return out
  })

  /** Breadcrumb at the cursor: document (in multi-doc files), then keys. */
  crumbs = $derived.by<string[]>(() => {
    const names: string[] = []
    for (let r = this.current; r; r = r.parent) names.unshift(r.sym.name)
    const docs = this.analysis?.docs ?? []
    if (docs.length > 1 && this.current) {
      const d = docs.find((x) => String(x.index) === this.current!.id.split('/')[0])
      if (d) names.unshift(d.name || `Document ${d.index + 1}`)
    }
    return names
  })

  /** The innermost symbol at the cursor, or null. Block entries own
   *  their whole lines (indentation included), like the engine's paths. */
  current = $derived.by<SymbolRef | null>(() => {
    const { line, col } = this.cursor
    let best: SymbolRef | null = null
    for (const ref of this.symbols) {
      const { start, end } = ref.sym.range
      if (line < start.line || line > end.line) continue
      if (line === start.line && col < start.col && best && best.sym.range.start.line === line) continue
      // Later (and deeper) matches win: symbols are in document order.
      best = ref
    }
    return best
  })

  /** Index of the document at the cursor. */
  currentDoc = $derived.by(() => {
    let idx = 0
    for (const d of this.analysis?.docs ?? []) if (d.range.start.line <= this.cursor.line) idx = d.index
    return idx
  })

  counts = $derived.by(() => {
    const c = { error: 0, warning: 0, info: 0 }
    for (const d of this.analysis?.diagnostics ?? []) c[d.severity]++
    return c
  })
}
