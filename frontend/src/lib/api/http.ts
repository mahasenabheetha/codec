import { request } from './client'

// The Network tool's HTTP tab: POST /api/v2/http/{kind}. Only text is
// rewritten; no request is ever sent and nothing is stored.

const post = <T>(kind: string, input: string) => request<T>('POST', '/api/v2/http/' + kind, { input })

export interface HTTPRequest {
  method: string
  url: string
  headers: { name: string; value: string }[]
  body?: string
  form?: { name: string; value: string; file?: boolean }[]
  timeoutMs?: number
  /** What didn't carry over, in words. */
  notes: string[]
}

export interface HTTPStatus {
  code: number
  name: string
  meaning: string
  cause: string
  retry: string
}

type Converted = { request: HTTPRequest; output: string }

export const curlToFetch = (input: string) => post<Converted>('curl', input)
export const fetchToCurl = (input: string) => post<Converted>('fetch', input)
export const statuses = (q: string) => post<{ statuses: HTTPStatus[] }>('status', q)
