// The Test data tab's jobs (internal/web/fake_api.go): lorem ipsum and
// rows of made-up data. A seed repeats a run; nothing is stored.
import { request } from './client'

export interface FakeColumn {
  name: string
  kind: string
  min?: string
  max?: string
  decimals?: number
  choices?: string[]
  /** Share of rows left empty, 0 to 100. */
  blank?: number
}

export type FakeFormat = 'json' | 'ndjson' | 'csv' | 'yaml' | 'sql'

export const fakeKinds = () => request<{ kinds: { Kind: string; About: string }[]; presets: Record<string, FakeColumn[]>; formats: FakeFormat[] }>('GET', '/api/v2/fake/kinds')

/** seed 0 picks one; the seed used comes back. */
export const lorem = (unit: 'words' | 'sentences' | 'paragraphs', count: number, classic: boolean, seed = 0) =>
  request<{ output: string; seed: number }>('POST', '/api/v2/fake/lorem', { unit, count, classic, seed })

export const fakeRows = (columns: FakeColumn[], count: number, format: FakeFormat, table: string, seed = 0) =>
  request<{ output: string; seed: number }>('POST', '/api/v2/fake/rows', { columns, count, format, table, seed })
