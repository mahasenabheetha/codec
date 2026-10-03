import { request } from './client'

// POST /api/v2/regex: test, replace preview and explanation in one call.
// Positions are UTF-16 offsets, as CodeMirror counts. Nothing is stored.

export type Style = 'go' | 'pcre'

export interface RegexGroup {
  number: number
  name?: string
  start: number // -1 when the group took no part
  end: number
  text: string
}

export interface RegexMatch {
  start: number
  end: number
  line: number
  text: string
  groups: RegexGroup[] | null
}

export interface RegexNode {
  kind: string
  start: number
  end: number
  text: string
  desc: string
  group?: number // a capture group's number
  children?: RegexNode[]
}

export interface RegexResponse {
  explain: RegexNode[]
  result?: { matches: RegexMatch[]; groups: string[]; truncated: boolean; timedOut: boolean }
  replaced?: string
  error?: { message: string; hint?: string }
}

export interface RegexRequest {
  pattern: string
  style: Style
  flags: string
  all: boolean
  text: string
  replace?: string
}

export const runRegex = (req: RegexRequest) => request<RegexResponse>('POST', '/api/v2/regex', req)
