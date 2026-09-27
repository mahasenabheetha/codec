import { request } from './client'

// Editor endpoints. `content` is the what-if buffer; omit it to use the
// file on disk. Lines and columns are 1-based.

export interface Pos {
  offset: number
  line: number
  col: number
}

export interface Range {
  start: Pos
  end: Pos
}

export interface Diagnostic {
  severity: 'error' | 'warning' | 'info'
  code: string
  message: string
  hint?: string
  why?: string // why the rule exists (lint findings)
  source?: string // "" syntax, style, kubernetes, deprecation, schema
  range: Range
}

export interface Expression {
  range: Range
  text: string
  syntax: string // go-template, jinja, github, argo
  phase: 'template' | 'runtime'
  standalone?: boolean
}

export interface YamlSymbol {
  name: string
  detail?: string
  kind: string // map, seq, scalar, alias; document when a big file is outlined by document only
  range: Range
  children?: YamlSymbol[]
}

export interface DocSummary {
  index: number
  name?: string
  range: Range
  symbols: YamlSymbol[]
  schema?: SchemaStatus
}

export interface SchemaStatus {
  title: string
  url: string
  state: 'ok' | 'pending' | 'none' | 'unavailable'
  message?: string
}

export interface Analysis {
  type: string
  typeTitle: string
  docs: DocSummary[]
  diagnostics: Diagnostic[]
  expressions: Expression[]
  /** Deeper levels left out: the file is too big for a full outline. */
  outlineTrimmed?: boolean
}

export interface Hover {
  range: Range
  title: string
  rows?: { label: string; value: string }[]
  code?: string
}

export interface Location {
  path?: string
  range: Range
}

export interface Completion {
  label: string
  insert?: string // text to insert; default label
  detail?: string
  doc?: string
  kind: string
  range: Range
}

export interface PathInfo {
  formats: Partial<Record<'dot' | 'yq' | 'jsonpath' | 'helm' | 'set', string>>
}

interface At {
  path: string
  content?: string
  line: number
  col: number
  type?: string // file type from the last analysis
}

export const analyze = (path: string, content: string | undefined, signal?: AbortSignal) =>
  request<Analysis>('POST', '/api/v2/yaml/analyze', { path, content }, signal)

export const hoverAt = (at: At, signal?: AbortSignal) =>
  request<{ hover: Hover | null }>('POST', '/api/v2/yaml/hover', at, signal).then((r) => r.hover)

export const definitionAt = (at: At) =>
  request<{ locations: Location[] }>('POST', '/api/v2/yaml/definition', at).then((r) => r.locations)

export const completeAt = (at: At, signal?: AbortSignal) =>
  request<{ items: Completion[] }>('POST', '/api/v2/yaml/complete', at, signal).then((r) => r.items)

export const pathAt = (at: At) => request<PathInfo>('POST', '/api/v2/yaml/path', at)

export const diffFile = (path: string, content: string) =>
  request<{ diff: string; changed: boolean }>('POST', '/api/v2/files/diff', { path, content })
