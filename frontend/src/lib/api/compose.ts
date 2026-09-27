import { request } from './client'
import type { Range } from './yaml'

export interface ComposeSource {
  file: string
  line: number
  range: Range
}

export interface ComposeLine {
  text: string
  from?: string // "extends web"; absent when written in source.file itself
  source?: ComposeSource
}

export interface ComposeService {
  name: string
  image?: string
  build?: string
  command?: string
  profiles: string[]
  dependsOn: { service: string; condition?: string; optional?: boolean; source?: ComposeSource }[]
  networks: string[]
  networkMode?: string
  extends?: string
  restart?: string
  replicas?: string
  healthcheck?: string
  envFiles: string[]
  environment: { name: string; value: string }[]
  source?: ComposeSource
  files: string[]
  effective: ComposeLine[]
}

export interface ComposeResource {
  name: string
  detail?: string
  external?: boolean
  implicit?: boolean
  usedBy: string[]
  source?: ComposeSource
}

export interface ComposeVariable {
  name: string
  value: string
  set: boolean
  from?: string // "what-if", the env file, "default"
  default?: string
  required?: boolean
  defined?: ComposeSource
  uses: ComposeSource[]
}

export interface ComposeProject {
  name?: string
  files: { path: string; role: 'layer' | 'include' | 'extends' | 'env'; read: boolean }[]
  candidates: string[]
  envFile?: string
  services: ComposeService[]
  networks: ComposeResource[]
  volumes: ComposeResource[]
  secrets: ComposeResource[]
  configs: ComposeResource[]
  variables: ComposeVariable[]
  edges: { from: string; to: string; kind: string; label?: string }[]
  ports: { service: string; hostIp?: string; published?: string; target: string; protocol: string; layer?: string; source?: ComposeSource }[]
  mounts: { service: string; type: string; from?: string; target: string; readOnly?: boolean; layer?: string; source?: ComposeSource }[]
  problems: { severity: 'error' | 'warning' | 'info'; code: string; message: string; hint?: string; service?: string; source?: ComposeSource }[]
}

export interface ComposeRequest {
  path: string
  files?: string[] // layers in merge order; empty = compose.yaml + override
  env?: Record<string, string> // what-if values, never saved
  envFile?: string // "" = .env, "none" = no env file
  overrides?: Record<string, string>
}

export const analyzeCompose = (req: ComposeRequest, signal?: AbortSignal) => request<ComposeProject>('POST', '/api/v2/compose/analyze', req, signal)
