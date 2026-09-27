// Helm state: the workspace's charts, and one render session per open
// Helm tab (layers, --set, release options, latest result).

import { isAbort } from '../../lib/api/client'
import { listCharts, renderChart, saveProfiles, type HelmChart, type HelmProfile, type HelmResult } from '../../lib/api/helm'
import { layout } from '../../lib/stores/layout.svelte'
import { openSessions } from '../editor/active.svelte'
import { workspace } from '../workspace/workspace.svelte'

class Charts {
  list = $state.raw<HelmChart[]>([])
  loaded = $state(false)

  async load() {
    if (!workspace.info?.open) {
      this.list = []
      return
    }
    try {
      this.list = await listCharts()
      this.loaded = true
    } catch {
      this.list = []
    }
  }

  byPath(path: string): HelmChart | undefined {
    return this.list.find((c) => c.path === path)
  }

  /** The chart that contains a workspace file, if any. */
  of(file: string): HelmChart | undefined {
    let best: HelmChart | undefined
    for (const c of this.list) {
      const inside = c.path === '.' || file === c.path || file.startsWith(c.path + '/')
      if (inside && (!best || c.path.length > best.path.length)) best = c
    }
    return best
  }
}

export const charts = new Charts()

/** Is this path inside the chart directory? */
export function inChart(chart: string, path: string): boolean {
  return chart === '.' || path.startsWith(chart + '/')
}

export interface LayerRow {
  path: string
  on: boolean
}

export class HelmSession {
  readonly chart: string

  layers = $state<LayerRow[]>([])
  setText = $state('') // one --set per line
  release = $state('')
  namespace = $state('')
  kubeVersion = $state('')
  profile = $state('') // active profile name, "" = unsaved
  inline = $state<Record<string, string>>({}) // virtual layers (INLINE_LAYER paths) -> values text

  result = $state.raw<HelmResult | null>(null)
  error = $state<string | null>(null)
  rendering = $state(false)

  private timer: ReturnType<typeof setTimeout> | undefined
  private inflight: AbortController | null = null
  /** Renders repeated while schemas download; bounded so a slow
   *  network can't keep it going. */
  private pendingTries = 0

  constructor(chart: string) {
    this.chart = chart
  }

  get info() {
    return charts.byPath(this.chart)
  }

  sets = $derived(
    this.setText
      .split('\n')
      .map((s) => s.trim())
      .filter((s) => s && !s.startsWith('#')),
  )

  /** What-if buffers that affect this render: files in the chart, and
   *  values files in use. */
  overrides = $derived.by(() => {
    const out: Record<string, string> = {}
    const used = new Set(this.layers.filter((l) => l.on).map((l) => l.path))
    for (const [path, s] of openSessions) {
      if (s.dirty && (inChart(this.chart, path) || used.has(path))) out[path] = s.buffer
    }
    for (const [path, text] of Object.entries(this.inline)) if (used.has(path)) out[path] = text
    return out
  })

  /** Load a profile (or the defaults) into the editable state. */
  apply(name: string) {
    const p: HelmProfile = (name && this.info?.profiles[name]) || {}
    this.profile = name
    const on = p.values ?? []
    this.inline = {}
    this.layers = on.map((path) => ({ path, on: true }))
    this.setText = (p.set ?? []).join('\n')
    this.release = p.release ?? ''
    this.namespace = p.namespace ?? ''
    this.kubeVersion = p.kubeVersion ?? ''
  }

  /** Start from a preset (not a saved profile). */
  applyPreset(p: HelmPreset) {
    this.profile = ''
    const layers = p.values.map((path) => ({ path, on: true }))
    this.inline = {}
    if (p.inline) {
      const path = INLINE_LAYER + p.inline.name
      this.inline = { [path]: p.inline.text }
      layers.push({ path, on: true })
    }
    this.layers = layers
    this.setText = p.set.join('\n')
    this.release = p.release ?? ''
    this.namespace = p.namespace ?? ''
    this.kubeVersion = ''
  }

  current(): HelmProfile {
    return {
      values: this.layers.filter((l) => l.on && !l.path.startsWith(INLINE_LAYER)).map((l) => l.path),
      set: this.sets,
      release: this.release || undefined,
      namespace: this.namespace || undefined,
      kubeVersion: this.kubeVersion || undefined,
    }
  }

  async saveAs(name: string) {
    if (!this.info) return
    const profiles = { ...this.info.profiles, [name]: this.current() }
    await saveProfiles(this.chart, profiles, name)
    this.profile = name
    await charts.load()
  }

  async deleteProfile(name: string) {
    if (!this.info) return
    const profiles = { ...this.info.profiles }
    delete profiles[name]
    await saveProfiles(this.chart, profiles, '')
    this.profile = ''
    await charts.load()
  }

  scheduleRender(delay = 200) {
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.render(), delay)
  }

  async render() {
    this.inflight?.abort()
    const ctrl = new AbortController()
    this.inflight = ctrl
    this.rendering = true
    try {
      const r = await renderChart(
        {
          chart: this.chart,
          values: this.layers.filter((l) => l.on).map((l) => l.path),
          set: this.sets,
          overrides: this.overrides,
          release: this.release,
          namespace: this.namespace,
          kubeVersion: this.kubeVersion,
        },
        ctrl.signal,
      )
      if (ctrl.signal.aborted) return
      this.result = r
      this.error = null
      // A first render doesn't wait for schema downloads; fetch the
      // complete findings once they are in.
      if (r.schemasPending && this.pendingTries < 10) {
        this.pendingTries++
        this.scheduleRender(1500)
      } else if (!r.schemasPending) this.pendingTries = 0
    } catch (e) {
      if (!isAbort(e)) this.error = e instanceof Error ? e.message : String(e)
    } finally {
      if (this.inflight === ctrl) this.rendering = false
    }
  }

  dispose() {
    clearTimeout(this.timer)
    this.inflight?.abort()
  }
}

/** Values, --set and release options to start a Helm view with, e.g.
 *  from an Argo CD application. inline is a values layer that isn't a
 *  file (the application's own values). */
export interface HelmPreset {
  values: string[]
  set: string[]
  release?: string
  namespace?: string
  inline?: { name: string; text: string }
}

/** Virtual layer paths (inline values) start with this. */
export const INLINE_LAYER = 'inline:'

/** A request to show a line of a chart's rendered output (e.g. a query
 *  result), to start from a preset, or to show a tab; the chart's Helm
 *  view takes it once it is open. */
class HelmNav {
  pending = $state<{ chart: string; line: number } | null>(null)
  preset = $state<{ chart: string; preset: HelmPreset } | null>(null)
  tab = $state<{ chart: string; tab: 'argo'; focus?: string } | null>(null)

  request(chart: string, line: number) {
    this.pending = { chart, line }
    layout.openHelm(chart)
  }

  preload(chart: string, preset: HelmPreset) {
    this.preset = { chart, preset }
    layout.openHelm(chart)
  }

  showArgo(chart: string, focus?: string) {
    this.tab = { chart, tab: 'argo', focus }
    layout.openHelm(chart)
  }
}

export const helmNav = new HelmNav()
