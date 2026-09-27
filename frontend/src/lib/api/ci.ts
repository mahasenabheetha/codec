import { request } from './client'
import type { Range } from './yaml'

export interface CISource {
  file?: string // workspace path; absent = the pipeline file
  line: number
  range: Range
}

export interface CIStep {
  id?: string
  name: string
  kind: string // run, uses, script, task, checkout, template, section, …
  detail?: string
  if?: string
  source: CISource
  from?: string
  children?: CIStep[]
  note?: string
}

export interface CICombo {
  name?: string
  values: { key: string; value: string }[]
  origin?: '' | 'include' | 'added'
}

export interface CIMatrix {
  axes: { name: string; values: string[] }[]
  include?: CICombo[]
  exclude?: CICombo[]
  combos: CICombo[]
  runtime?: string
  maxParallel?: string
  failFast?: string
}

export interface CILine {
  text: string
  from?: string
  source?: CISource
}

export interface CIJob {
  id: string
  name: string
  stage?: string
  kind: 'job' | 'reusable' | 'trigger' | 'deployment' | 'template'
  group?: string
  source: CISource
  needs: { job: string; optional?: boolean; note?: string; source?: CISource }[]
  if?: string
  when?: string
  rules?: string[]
  runner?: string
  environment?: string
  allowFailure?: boolean
  matrix?: CIMatrix
  instances?: string[]
  steps: CIStep[]
  outputs?: { key: string; value: string }[]
  extends?: string[]
  uses?: string
  with?: { name: string; value: string; state: 'passed' | 'default' | 'missing' | 'unknown'; type?: string; required?: boolean }[]
  from?: string
  effective: CILine[]
}

export interface CIInput {
  name: string
  type?: string
  default?: string
  hasDefault?: boolean
  required?: boolean
  description?: string
  options?: string[]
  from?: string
  source: CISource
}

export interface CIPipeline {
  tool: 'github-actions' | 'gitlab-ci' | 'azure-pipelines'
  file: string
  name?: string
  kind: 'workflow' | 'action' | 'template' | 'reusable'
  triggers: string[]
  inputs: CIInput[]
  variables: { name: string; value: string; scope: string; source: CISource }[]
  stages: { name: string; title?: string; jobs: string[]; dependsOn?: string[]; if?: string; source?: CISource; from?: string; defined: boolean }[]
  jobs: CIJob[]
  edges: { from: string; to: string; label?: string }[]
  includes: { kind: string; target: string; state: 'resolved' | 'unresolved' | 'missing'; file?: string; note?: string; source: CISource }[]
  templates: { name: string; source: CISource }[]
  problems: { severity: 'error' | 'warning' | 'info'; code: string; message: string; hint?: string; job?: string; source?: CISource }[]
}

export interface CIRequest {
  path: string
  params?: Record<string, string>
  overrides?: Record<string, string>
}

export const analyzeCI = (req: CIRequest, signal?: AbortSignal) => request<CIPipeline>('POST', '/api/v2/ci/analyze', req, signal)
