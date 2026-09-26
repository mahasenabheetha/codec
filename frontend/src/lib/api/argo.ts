import { request } from './client'
import type { Range } from './yaml'

export interface ArgoSource {
  file?: string // workspace path; absent for a render
  doc: number
  line: number
  range: Range
}

export interface ArgoTrigger {
  sensor: string
  name: string
  operation?: string
}

export interface ArgoPart {
  text: string
  kind?: 'resolved' | 'runtime' | 'expr' | 'missing' | 'template'
  expr?: string
  note?: string
}

export interface ArgoValue {
  name?: string
  text: string
  state: 'known' | 'runtime' | 'missing' | 'template'
  parts?: ArgoPart[]
  from?: string
  enum?: string[]
  hint?: string
}

export interface ArgoNode {
  id: string
  parent?: string
  name: string
  template?: string
  ref?: string
  type?: string
  group?: number
  inputs: ArgoValue[]
  children?: string[]
  edges?: { from: string; to: string; label?: string }[]
  when?: { value: ArgoValue; result?: 'true' | 'false'; error?: string }
  depends?: string
  loop?: string
  item?: ArgoValue
  continueOn?: string
  image?: string
  action?: string
  daemon?: boolean
  outputs?: string[]
  def?: ArgoSource
  call?: ArgoSource
  error?: string
  note?: string
  body?: ArgoPart[]
}

export interface ArgoProblem {
  severity: 'error' | 'warning' | 'info'
  code: string
  message: string
  hint?: string
  node?: string
  source?: ArgoSource
}

export interface ArgoResult {
  workflow: { kind: string; name: string; source: ArgoSource; schedule?: string; via?: ArgoTrigger }
  base?: { kind: string; name: string; source: ArgoSource }
  params: ArgoValue[]
  nodes: ArgoNode[]
  roots: string[]
  problems: ArgoProblem[]
}

export interface ArgoWorkflow {
  key: string
  kind: string
  name: string
  via?: ArgoTrigger
  schedule?: string
  source: ArgoSource
}

export interface ArgoApp {
  kind: 'Application' | 'ApplicationSet'
  name: string
  namespace?: string
  project?: string
  source: ArgoSource
  sources: {
    repoURL?: string
    path?: string
    chart?: string
    revision?: string
    ref?: string
    tool: string
    helm?: { releaseName?: string; valueFiles?: string[]; parameters?: string[]; values?: string }
    local?: { chart: string; values?: string[]; missing?: string[]; match: 'path' | 'name' }
  }[]
  destination: { server?: string; name?: string; namespace?: string }
  sync?: string
  generators?: string[]
}

export interface ArgoAnalysis {
  workflows: ArgoWorkflow[]
  apps: ArgoApp[]
  result: ArgoResult | null
  params: Record<string, string>
  error?: string
}

export interface ArgoScope {
  kind: 'file' | 'text'
  path?: string
  text?: string
  workflow?: string
  params?: Record<string, string>
  paramFile?: string
  overrides?: Record<string, string>
}

export const analyzeArgo = (scope: ArgoScope, signal?: AbortSignal) =>
  request<ArgoAnalysis>('POST', '/api/v2/argo/analyze', scope, signal)
