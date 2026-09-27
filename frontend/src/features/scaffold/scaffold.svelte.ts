// State of the New view: the starters, the form values (kept for this
// session only, never saved), the generated files with any what-if
// edits, and what codec found in them.

import { isAbort } from '../../lib/api/client'
import {
  checkFiles,
  listStarters,
  renderStarter,
  type CheckResult,
  type GeneratedFile,
  type Starter,
  type StarterList,
} from '../../lib/api/scaffold'

/** Workspace-looking path for editor calls on a generated file: type
 *  detection sees the real name, and nothing in the folder matches. */
export const virtualPath = (p: string) => `~new/${p}`

class NewFiles {
  list = $state.raw<StarterList | null>(null)
  loadError = $state('')
  selected = $state('')
  /** Form values per starter id. */
  values = $state<Record<string, Record<string, string>>>({})
  files = $state<GeneratedFile[]>([])
  checked = $state.raw<CheckResult | null>(null)
  fieldErrors = $state<Record<string, string>>({})
  renderError = $state('')
  /** Index into files, or -1 for the rendered chart. */
  active = $state(0)
  edited = $state(false)
  busy = $state(false)

  starter = $derived<Starter | undefined>(this.list?.starters.find((s) => s.id === this.selected))

  private inflight: AbortController | null = null
  private timer: ReturnType<typeof setTimeout> | undefined

  async load() {
    try {
      this.list = await listStarters()
      this.loadError = ''
      if (!this.starter && this.list.starters.length) this.select(this.list.starters[0].id)
    } catch (e) {
      this.loadError = e instanceof Error ? e.message : String(e)
    }
  }

  select(id: string) {
    this.selected = id
    const s = this.starter
    if (s && !this.values[id]) {
      const v: Record<string, string> = {}
      for (const f of s.fields) v[f.name] = f.default ?? ''
      this.values[id] = v
    }
    this.active = 0
    this.render()
  }

  /** Current values of the selected starter. */
  get form(): Record<string, string> {
    return this.values[this.selected] ?? {}
  }

  set(name: string, value: string) {
    this.values[this.selected][name] = value
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.render(), 250)
  }

  reset() {
    delete this.values[this.selected]
    this.select(this.selected)
  }

  /** Fill the starter in again; what-if edits in the preview are
   *  replaced by the new output. */
  async render() {
    const id = this.selected
    if (!id) return
    this.inflight?.abort()
    const ctl = (this.inflight = new AbortController())
    this.busy = true
    try {
      const r = await renderStarter(id, $state.snapshot(this.form), ctl.signal)
      if (ctl.signal.aborted || id !== this.selected) return
      this.renderError = ''
      this.fieldErrors = Object.fromEntries((r.fieldErrors ?? []).map((e) => [e.field, e.message]))
      if (r.fieldErrors?.length) return // keep the last good output on screen
      this.checked = r
      this.files = r.files.map((f) => ({ path: f.path, content: f.content }))
      this.edited = false
      if (this.active >= this.files.length) this.active = 0
    } catch (e) {
      if (!isAbort(e)) this.renderError = e instanceof Error ? e.message : String(e)
    } finally {
      if (this.inflight === ctl) this.busy = false
    }
  }

  /** A what-if edit in the preview: check the files again shortly. */
  edit(index: number, content: string) {
    if (this.files[index]?.content === content) return
    this.files[index] = { ...this.files[index], content }
    this.edited = true
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.recheck(), 500)
  }

  private async recheck() {
    this.inflight?.abort()
    const ctl = (this.inflight = new AbortController())
    try {
      const r = await checkFiles($state.snapshot(this.files), ctl.signal)
      if (!ctl.signal.aborted) this.checked = r
    } catch (e) {
      if (!isAbort(e)) this.renderError = e instanceof Error ? e.message : String(e)
    }
  }

  /** Findings per generated file (by path), for the tab badges. */
  counts(path: string): { errors: number; warnings: number } {
    const f = this.checked?.files.find((c) => c.path === path)
    const ds = f?.diagnostics ?? []
    const chart = (this.checked?.charts ?? []).flatMap((c) => c.diagnostics).filter((d) => d.file === path)
    const all = [...ds, ...chart]
    return {
      errors: all.filter((d) => d.severity === 'error').length,
      warnings: all.filter((d) => d.severity === 'warning').length,
    }
  }
}

export const newFiles = new NewFiles()
