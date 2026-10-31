// The JSON & YAML tool's jobs (internal/web/json_api.go). Input is read
// as JSON, JSONC or JSON5 and written as strict JSON.
import { request } from './client'

export type Dialect = 'json' | 'jsonc' | 'json5'

export interface JsonOptions {
  /** '' = two spaces, 'min' = one line, else spaces or a tab. */
  indent?: string
  sort?: boolean
}

export interface Change {
  path: string
  kind: 'added' | 'removed' | 'changed' | 'type'
  old?: string
  new?: string
}

export interface Leaf {
  path: string
  value: string
}

export interface Stats {
  bytes: number
  values: number
  objects: number
  arrays: number
  keys: number
  strings: number
  numbers: number
  bools: number
  nulls: number
  maxDepth: number
}

const post = <T>(kind: string, body: Record<string, unknown>) => request<T>('POST', `/api/v2/json/${kind}`, body)

export const formatJSON = (input: string, o: JsonOptions = {}) => post<{ output: string; dialect: Dialect }>('format', { input, ...o })
export const toYAML = (input: string) => post<{ output: string; dialect: Dialect }>('to-yaml', { input })
export const fromYAML = (input: string, o: JsonOptions = {}) => post<{ output: string }>('from-yaml', { input, ...o })
export const flattenJSON = (input: string) => post<{ output: string; leaves: Leaf[]; dialect: Dialect }>('flatten', { input })
export const unflattenJSON = (input: string, o: JsonOptions = {}) => post<{ output: string }>('unflatten', { input, ...o })
export const diffJSON = (input: string, other: string) => post<{ changes: Change[] }>('diff', { input, other })
export const inferSchema = (input: string, o: JsonOptions = {}) => post<{ output: string; samples: number }>('schema', { input, ...o })
export const queryJSON = (input: string, expr: string, o: JsonOptions = {}) => post<{ results: string[]; truncated?: boolean }>('query', { input, expr, ...o })
export const jsonStats = (input: string) => post<Stats>('stats', { input })
