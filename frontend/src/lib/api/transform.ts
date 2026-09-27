// Types and call for POST /api/transform (internal/web/server.go).

import { request } from './client'

export type Mode =
  | 'auto'
  | 'b64-encode'
  | 'b64-decode'
  | 'json-pretty'
  | 'json-min'
  | 'validate'
  | 'jwt'
  | 'ansible'

export type Kind = 'json' | 'jwt' | 'base64' | 'ansible' | 'unknown'

export interface TransformRequest {
  input: string
  mode: Mode
  urlSafe?: boolean
  indent?: string
}

export interface AnsibleLine {
  text: string
  level?: 'error' | 'warn'
}

export interface AnsibleSection {
  title: string
  lines: AnsibleLine[]
}

export interface AnsibleTask {
  name?: string
  path?: string
  host: string
  item?: string
  status: string
  retries?: number
  cause?: { text: string; source: string }
  summary?: { key: string; value: string; bad?: boolean }[]
  sections?: AnsibleSection[]
}

export interface JwtParts {
  header: string // pretty JSON
  payload: string // pretty JSON
  signature: string // base64url, not verified
}

export interface TransformResponse {
  output: string
  kind: Kind
  task?: AnsibleTask
  jwt?: JwtParts
}

export function transform(req: TransformRequest): Promise<TransformResponse> {
  return request<TransformResponse>('POST', '/api/transform', req)
}
