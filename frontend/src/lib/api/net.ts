import { request } from './client'
import type { URLParts } from './encode'

// The Network tool: POST /api/v2/net/{kind}. Computed locally; nothing
// is looked up on the network and nothing is stored.

const post = <T>(kind: string, body: object) => request<T>('POST', '/api/v2/net/' + kind, body)

/** A CIDR range in detail; counts are decimal strings (IPv6 is big). */
export interface Range {
  cidr: string
  version: 4 | 6
  prefix: number
  network: string
  broadcast?: string
  first: string
  last: string
  mask: string
  wildcard: string
  addresses: string
  hosts: string
  kind: string
  /** The address given, when it isn't the network address. */
  host?: string
}

export interface Subnets {
  count: string
  subnets: string[]
  truncated?: boolean
  each: Range
}

export interface Listed {
  input: string
  /** The line's # comment, e.g. "hub VNet". */
  note?: string
  cidr?: string
  error?: string
}

/** Two ranges of the list by index: A holds B ("contains"), or both are one. */
export interface Overlap {
  a: number
  b: number
  relation: 'same' | 'contains'
}

export interface IPInfo {
  address: string
  version: 4 | 6
  expanded?: string
  integer: string
  hex: string
  binary: string
  mapped?: string
  reverse: string
  kind: string
}

export const cidrDetails = (input: string) => post<{ range: Range }>('cidr', { input })
export const cidrSplit = (input: string, prefix: number) => post<Subnets>('cidr', { input, op: 'split', prefix })
export const cidrContains = (input: string, ip: string) => post<{ contains: boolean }>('cidr', { input, op: 'contains', ip })
export const overlaps = (lines: string[]) => post<{ ranges: Listed[]; overlaps: Overlap[] }>('overlaps', { lines })
export const ipInfo = (input: string) => post<{ ip: IPInfo }>('ip', { input })
export const urlParse = (input: string) => post<{ parts: URLParts }>('url', { op: 'parse', input })
export const urlBuild = (parts: URLParts) => post<{ url: string }>('url', { op: 'build', parts })

/** 1048576 → "1,048,576"; works past 2^53 (IPv6 counts). */
export function groupDigits(n: string): string {
  try {
    return BigInt(n).toLocaleString('en-US')
  } catch {
    return n
  }
}
