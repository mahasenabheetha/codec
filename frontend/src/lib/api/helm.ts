import { request } from './client'

export interface HelmProfile {
  values?: string[]
  set?: string[]
  release?: string
  namespace?: string
  kubeVersion?: string
}

export interface HelmChart {
  path: string // workspace-relative chart dir; "." = root
  name: string
  version?: string
  candidates: string[] // values files that can be layered
  profiles: Record<string, HelmProfile>
  active?: string
}

export interface HelmDoc {
  source: string // e.g. "app/templates/service.yaml"
  kind?: string
  name?: string
  hook?: boolean
  line: number // line of "---" in the manifest
}

export interface HelmDiagnostic {
  severity: 'error' | 'warning' | 'info'
  code: string
  message: string
  hint?: string
  file?: string // workspace path, layer name or "--set"
  line?: number
  col?: number
}

export interface Origin {
  layer: string
  kind: 'set' | 'file' | 'default' | 'global' | 'computed'
  line?: number
  value?: unknown
  deleted?: boolean
}

export interface HelmResult {
  chart: { name: string; version: string; appVersion?: string; dependencies?: string[] }
  manifest: string
  docs: HelmDoc[]
  notes?: string
  values: Record<string, unknown>
  provenance: Record<string, Origin[]>
  diagnostics: HelmDiagnostic[]
  durationNs: number
  valuesYAML: string
  valuesLines: Record<string, string> // line number -> path
}

export interface RenderRequest {
  chart: string
  values: string[]
  set: string[]
  overrides: Record<string, string>
  release?: string
  namespace?: string
  kubeVersion?: string
}

export const listCharts = () => request<{ charts: HelmChart[] }>('GET', '/api/v2/helm/charts').then((r) => r.charts)

export const renderChart = (req: RenderRequest, signal?: AbortSignal) =>
  request<HelmResult>('POST', '/api/v2/helm/render', req, signal)

export const saveProfiles = (chart: string, profiles: Record<string, HelmProfile>, active: string) =>
  request<{ ok: boolean }>('POST', '/api/v2/helm/profiles', { chart, profiles, active })
