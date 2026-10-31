// The CSV & TSV tab's jobs (internal/web/table_api.go). Nothing is stored.
import { request } from './client'

export type Delimiter = '' | ',' | '\t' | ';' | '|'
export type HeaderMode = 'auto' | 'yes' | 'no'
export type ColumnType = 'integer' | 'number' | 'boolean' | 'date' | 'text' | 'empty'
export type TableFormat = 'json' | 'jsonl' | 'yaml' | 'markdown' | 'csv' | 'tsv'

export interface TableColumn {
  name: string
  type: ColumnType
  filled: number
  empty: number
  distinct: number
  /** More distinct values than were counted (10 000). */
  distinctMore?: boolean
}

export interface TableInfo {
  delimiter: Exclude<Delimiter, ''>
  header: boolean
  delimGuessed: boolean
  headerGuessed: boolean
  columns: TableColumn[]
  /** Rows in the input; the viewer gets at most 100 000 of them. */
  total: number
  /** Rows with more or fewer fields than the rest. */
  ragged: number
}

export interface TableRead {
  delimiter?: Delimiter
  header?: HeaderMode
}

const post = <T>(kind: string, body: Record<string, unknown>) => request<T>('POST', `/api/v2/table/${kind}`, body)

export const parseTable = (input: string, o: TableRead = {}) => post<TableInfo & { rows: string[][] }>('parse', { input, ...o })

export const convertTable = (input: string, to: TableFormat, o: TableRead & { typed?: boolean; indent?: string } = {}) =>
  post<TableInfo & { output: string }>('convert', { input, to, ...o })

export const tableFromJSON = (input: string, to: 'csv' | 'tsv') => post<{ output: string; columns: TableColumn[]; total: number }>('from-json', { input, to })
