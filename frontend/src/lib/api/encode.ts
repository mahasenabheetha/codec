import { request } from './client'

// The Encode & hash jobs: POST /api/v2/encode/{kind}. Nothing is stored.

const post = <T>(kind: string, body: object) => request<T>('POST', '/api/v2/encode/' + kind, body)

export interface Param {
  key: string
  value: string
}

export interface URLParts {
  scheme?: string
  user?: string
  host?: string
  port?: string
  path?: string
  query: Param[]
  fragment?: string
}

export interface Digest {
  algorithm: string
  hex: string
  base64: string
  weak?: boolean
}

export interface SecretOptions {
  length: number
  format: 'text' | 'hex' | 'base64' | 'base64url'
  lower: boolean
  upper: boolean
  digits: boolean
  symbols: boolean
  noAmbiguous: boolean
}

export interface Secret {
  value: string
  bits: number
}

export const urlEncode = (input: string, whole: boolean, plus: boolean) =>
  post<{ output: string }>('url', { op: 'encode', input, whole, plus })
export const urlDecode = (input: string, plus: boolean) => post<{ output: string }>('url', { op: 'decode', input, plus })
export const urlParse = (input: string) => post<{ parts: URLParts }>('url', { op: 'parse', input })

export const hexEncode = (input: string, upper: boolean, sep: string) =>
  post<{ output: string }>('hex', { op: 'encode', input, upper, sep })
export const hexDecode = (input: string) => post<{ output: string; text: boolean; bytes: number }>('hex', { op: 'decode', input })

/** HMACs when key is a string (an empty key is valid); digests when null. */
export const hash = (input: string, key: string | null) => post<{ digests: Digest[]; hmac: boolean }>('hash', { input, key })

export const secrets = (secret: SecretOptions, count: number) => post<{ secrets: Secret[] }>('secret', { secret, count })
export const uuids = (count: number) => post<{ uuids: string[] }>('uuid', { count })

export const htpasswd = (user: string, password: string, cost: number) =>
  post<{ line: string }>('htpasswd', { op: 'make', user, password, cost })
export const htpasswdCheck = (line: string, password: string) =>
  post<{ match: boolean }>('htpasswd', { op: 'check', line, password })
