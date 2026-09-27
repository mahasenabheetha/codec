// Lint state: the user's rule levels and schema options (saved in
// codec's settings, never in the repo), and the last workspace lint.

import { isAbort } from '../../lib/api/client'
import {
  getLintSettings,
  lintWorkspace,
  saveLintSettings,
  type Level,
  type LintedFile,
  type LintRule,
  type LintSettings,
} from '../../lib/api/lint'
import { toast } from '../../lib/stores/toast.svelte'
import { openSessions } from '../editor/active.svelte'

class Lint {
  settings = $state<LintSettings>({})
  rules = $state.raw<LintRule[]>([])
  k8sVersions = $state.raw<string[]>([])
  defaultK8sVersion = $state('')
  loaded = $state(false)
  /** Bumped on every settings change, so views that lint re-run. */
  version = $state(0)

  // Workspace lint
  results = $state.raw<LintedFile[] | null>(null)
  checked = $state(0)
  durationMs = $state(0)
  running = $state(false)
  error = $state<string | null>(null)
  private inflight: AbortController | null = null

  async load() {
    try {
      const r = await getLintSettings()
      this.settings = r.settings ?? {}
      this.rules = r.rules
      this.k8sVersions = r.k8sVersions
      this.defaultK8sVersion = r.defaultK8sVersion
      this.loaded = true
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e)
    }
  }

  rule(id: string): LintRule | undefined {
    return this.rules.find((r) => r.id === id)
  }

  level(id: string): Level {
    return this.settings.rules?.[id] ?? this.rule(id)?.default ?? 'off'
  }

  /** Save a change and re-check what is open. */
  async update(patch: Partial<LintSettings>) {
    const next = { ...this.settings, ...patch }
    try {
      await saveLintSettings(next)
      this.settings = next
      this.version++
      for (const s of openSessions.values()) s.scheduleAnalyze(0)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }

  setLevel(id: string, level: Level) {
    const rules = { ...(this.settings.rules ?? {}) }
    if (level === this.rule(id)?.default) delete rules[id]
    else rules[id] = level
    return this.update({ rules })
  }

  /** Turn a rule off from a finding, with a way back. */
  async turnOff(id: string) {
    if (!this.loaded) await this.load()
    const title = this.rule(id)?.title ?? id
    await this.setLevel(id, 'off')
    toast(`Turned off "${title}". Change it back in Settings.`)
  }

  async runWorkspace() {
    this.inflight?.abort()
    const ctrl = new AbortController()
    this.inflight = ctrl
    this.running = true
    this.error = null
    try {
      const r = await lintWorkspace(ctrl.signal)
      this.results = r.files
      this.checked = r.checked
      this.durationMs = r.durationMs
    } catch (e) {
      if (!isAbort(e)) this.error = e instanceof Error ? e.message : String(e)
    } finally {
      if (this.inflight === ctrl) this.running = false
    }
  }
}

export const lint = new Lint()

export const groupTitles: Record<string, string> = {
  style: 'Style',
  kubernetes: 'Kubernetes',
  deprecation: 'Deprecated APIs',
  schema: 'Schemas',
}
