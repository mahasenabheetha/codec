import { request } from './client'
import type { Diagnostic } from './yaml'

export interface K8sSource {
  file?: string // workspace path; absent for a render
  doc: number
  line: number
}

export interface SecretValue {
  key: string
  value: string
  binary?: boolean
  size: number
  error?: string
  line: number
}

export interface K8sCard {
  id: string
  kind: string
  name: string
  namespace?: string
  source: K8sSource
  facts: { label: string; value: string }[]
  containers?: { name: string; image: string; ports?: string[]; init?: boolean }[]
  secret?: SecretValue[]
}

export interface K8sNode {
  id: string
  kind: string
  name: string
  namespace?: string
  missing?: boolean
  source?: K8sSource
}

export interface K8sEdge {
  from: string
  to: string
  kind: string
  label?: string
}

export interface K8sFinding {
  severity: 'warning' | 'info'
  code: string
  message: string
  hint?: string
  object: string
  source: K8sSource
}

export interface ResourceRow {
  object: string
  replicas: number
  requestsCpu: string
  requestsMemory: string
  limitsCpu: string
  limitsMemory: string
  missing?: boolean
}

export interface K8sInventory {
  objects: number
  kinds: { kind: string; count: number }[]
  images: { ref: string; repo: string; tag: string; pinned: boolean; usedBy: string[] }[]
  ports: { object: string; kind: string; name?: string; port: string; target?: string; protocol: string; nodePort?: string }[]
  config: { kind: string; name: string; found: boolean; usedBy: string[] }[]
  resources: { rows: ResourceRow[]; total: ResourceRow }
}

export interface K8sAnalysis {
  manifest?: string
  cards: K8sCard[]
  inventory: K8sInventory
  graph: { nodes: K8sNode[]; edges: K8sEdge[]; findings: K8sFinding[] }
  problems: Diagnostic[]
  error?: string
}

export interface K8sScope {
  kind: 'folder' | 'text' | 'kustomize' | 'helm'
  path?: string
  text?: string
  chart?: string
  profile?: string
  overrides?: Record<string, string>
}

export const analyzeK8s = (scope: K8sScope, signal?: AbortSignal) =>
  request<K8sAnalysis>('POST', '/api/v2/k8s/analyze', scope, signal)

export const neat = (path: string, content?: string) =>
  request<{ text?: string; error?: string }>('POST', '/api/v2/k8s/neat', { path, content })
