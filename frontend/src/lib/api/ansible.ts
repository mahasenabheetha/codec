import { request } from './client'
import type { Range } from './yaml'

export interface AnsibleSource {
  file: string
  line: number
  range: Range
}

export interface AnsibleTask {
  name: string
  kind: 'task' | 'block' | 'section' | 'role' | 'include' | 'import' | 'facts' | 'flush' | 'handler'
  module?: string
  detail?: string
  when?: string
  loop?: string
  notify: string[]
  listen: string[]
  tags: string[]
  register?: string
  role?: string
  dynamic?: boolean
  missing?: boolean
  source?: AnsibleSource
  children: AnsibleTask[]
}

export interface AnsiblePlay {
  name: string
  hosts?: string
  imported?: string
  source?: AnsibleSource
  varsFiles: string[]
  steps: AnsibleTask[]
  handlers: AnsibleTask[]
}

export interface AnsibleRole {
  name: string
  path?: string
  found: boolean
  external?: boolean
  elsewhere?: boolean
  parts: string[]
  dependencies: string[]
}

export interface AnsibleVar {
  name: string
  value: string
  kind: string
  scope?: string
  rank: number
  source?: AnsibleSource
}

export interface AnsibleProblem {
  severity: 'error' | 'warning' | 'info'
  code: string
  message: string
  hint?: string
  source?: AnsibleSource
}

export interface AnsiblePlaybook {
  file: string
  kind: 'playbook' | 'tasks'
  role?: string
  plays: AnsiblePlay[]
  tasks: AnsibleTask[]
  handlers: AnsibleTask[]
  roles: AnsibleRole[]
  variables: AnsibleVar[]
  files: { path: string; role: string; read: boolean }[]
  problems: AnsibleProblem[]
}

export interface AnsibleKV {
  name: string
  value: string
  source?: AnsibleSource
}

export interface AnsibleInventory {
  groups: { name: string; hosts: string[]; children: string[]; vars: AnsibleKV[]; source?: AnsibleSource }[]
  hosts: { name: string; count: number; groups: string[]; vars: AnsibleKV[]; source?: AnsibleSource }[]
  problems: AnsibleProblem[]
}

/** A playbook drawn as a diagram (internal/ansible/graph.go). */
export interface PlaybookGraph {
  nodes: PlaybookNode[]
  edges: { from: string; to: string; kind: 'role' | 'dependency' | 'import' | 'include' | 'notify'; dynamic?: boolean }[]
}

export interface PlaybookNode {
  id: string
  kind: 'play' | 'role' | 'tasks' | 'handler'
  label: string
  sub?: string
  file?: string
  line?: number
  role?: string
  play?: string
  tasks?: number
  external?: boolean
  missing?: boolean
  assumed?: boolean
  candidates?: string[]
}

export type AnsibleAnalysis = { kind: 'playbook'; playbook: AnsiblePlaybook; graph: PlaybookGraph } | { kind: 'inventory'; inventory: AnsibleInventory }

export interface TaskMatch {
  file: string
  line: number
  range: Range
  name: string
  role?: string
  why: string
  score: number
}

export const analyzeAnsible = (req: { path: string; overrides?: Record<string, string> }, signal?: AbortSignal) =>
  request<AnsibleAnalysis>('POST', '/api/v2/ansible/analyze', req, signal)

export const findTask = (req: { name: string; path?: string }, signal?: AbortSignal) =>
  request<{ matches: TaskMatch[] }>('POST', '/api/v2/ansible/find-task', req, signal)
