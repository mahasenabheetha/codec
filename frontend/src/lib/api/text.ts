import { request } from './client'

// The Text & data tool: POST /api/v2/text/{kind}. Nothing is stored.

export interface NameCase {
  name: string
  value: string
}

/** A name tested against one naming rule. */
export interface NameCheck {
  rule: string
  for: string
  valid: boolean
  problems: string[]
  /** The nearest name that passes; absent when it passes or nothing is left. */
  fixed?: string
}

export const caseOf = (input: string) => request<{ words: string[]; cases: NameCase[]; checks: NameCheck[] }>('POST', '/api/v2/text/case', { input })

/** Somewhere a value can be quoted for. */
export interface QuoteTarget {
  id: string
  name: string
  group: string
}

/** One way to write a value for a target, with what it does. */
export interface QuoteForm {
  label: string
  value: string
  why: string
}

/** A wrapping: the value so far quoted for target, after before. */
export interface QuoteLayer {
  target: string
  before: string
}

export const quoteTargets = (signal?: AbortSignal) => request<{ targets: QuoteTarget[] }>('GET', '/api/v2/text/quote/targets', undefined, signal)

export const quoteValue = (value: string, target: string, layers: QuoteLayer[], signal?: AbortSignal) =>
  request<{ forms: QuoteForm[]; notes: string[]; steps: { target: string; value: string }[] }>('POST', '/api/v2/text/quote', { value, target, layers }, signal)

export const unquoteValue = (value: string, target: string, signal?: AbortSignal) =>
  request<{ value: string }>('POST', '/api/v2/text/quote', { value, target, unquote: true }, signal)

/** What Sanitize takes out, and how. */
export interface SanitizeOptions {
  secrets: boolean
  emails: boolean
  ips: boolean
  keepPrivate: boolean
  hosts: boolean
  uuids: boolean
  azure: boolean
  users: boolean
  words: string[]
  mask: boolean
  /** Values found that stay as they are. */
  keep: string[]
}

/** Every kind on, stand-ins, nothing kept: the server's default. */
export const sanitizeAll = (): SanitizeOptions => ({
  secrets: true,
  emails: true,
  ips: true,
  keepPrivate: false,
  hosts: true,
  uuids: true,
  azure: true,
  users: true,
  words: [],
  mask: false,
  keep: [],
})

export interface SanitizeFinding {
  kind: 'secret' | 'email' | 'ip' | 'host' | 'uuid' | 'azure' | 'user' | 'word'
  value: string
  replacement: string
  count: number
  kept: boolean
}

export const sanitizeText = (text: string, options: SanitizeOptions, signal?: AbortSignal) =>
  request<{ text: string; findings: SanitizeFinding[] }>('POST', '/api/v2/text/sanitize', { text, options }, signal)
