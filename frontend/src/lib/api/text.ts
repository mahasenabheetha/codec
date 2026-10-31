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
