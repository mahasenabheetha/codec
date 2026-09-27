import { request, requestBlob } from './client'
import type { Diagnostic } from './yaml'

// Scaffolding: starters, their checked output, zip downloads and
// clone-with-rename. Nothing is ever written to disk by codec.

export interface Field {
  name: string
  label: string
  help?: string
  type: 'text' | 'bool' | 'choice'
  default?: string
  choices?: string[]
  pattern?: string
  optional?: boolean
  when?: string
}

export interface Starter {
  id: string
  title: string
  category: string
  description?: string
  personal?: boolean
  fields: Field[]
  files: string[]
}

export interface StarterList {
  starters: Starter[]
  personalDir: string
  problems: string[]
}

export interface GeneratedFile {
  path: string
  content: string
}

export interface CheckedFile extends GeneratedFile {
  type: string
  diagnostics: Diagnostic[]
}

export interface ChartDiagnostic {
  severity: 'error' | 'warning' | 'info'
  code: string
  message: string
  hint?: string
  file?: string
  line?: number
  col?: number
  manifest?: number
}

export interface ChartCheck {
  chart: string
  manifest: string
  notes?: string
  diagnostics: ChartDiagnostic[]
}

export interface CheckResult {
  files: CheckedFile[]
  charts: ChartCheck[]
  fieldErrors?: { field: string; message: string }[]
}

export function listStarters(): Promise<StarterList> {
  return request('GET', '/api/v2/scaffold/starters')
}

export function renderStarter(id: string, values: Record<string, string>, signal?: AbortSignal): Promise<CheckResult> {
  return request('POST', '/api/v2/scaffold/render', { id, values }, signal)
}

export function checkFiles(files: GeneratedFile[], signal?: AbortSignal): Promise<CheckResult> {
  return request('POST', '/api/v2/scaffold/check', { files }, signal)
}

export function zipFiles(name: string, files: GeneratedFile[]): Promise<Blob> {
  return requestBlob('/api/v2/scaffold/zip', { name, files })
}

export interface Change {
  id: string
  kind: 'rename' | 'suggest' | 'check'
  applied: boolean
  path: string
  key?: boolean
  line: number
  before: string
  after?: string
  reason?: string
}

export interface CloneResponse {
  error?: string
  from?: string // the name replaced by default
  docs: { index: number; name?: string }[]
  result?: { from: string; to: string; text: string; changes: Change[] }
  type?: string
  typeTitle?: string
  diagnostics?: Diagnostic[]
}

export function cloneFile(
  req: { path: string; content?: string; doc: number; from: string; to: string; flip: string[] },
  signal?: AbortSignal,
): Promise<CloneResponse> {
  return request('POST', '/api/v2/scaffold/clone', req, signal)
}
