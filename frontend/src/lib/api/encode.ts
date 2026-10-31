import { postRaw, request } from './client'

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
/** A password's rules: a class is used with a minimum of 0 or more, left
 *  out with -1; exclude takes characters out of every class. */
export interface PasswordRules {
  length: number
  minLower: number
  minUpper: number
  minDigits: number
  minSymbols: number
  noAmbiguous: boolean
  exclude: string
}
export interface PassphraseRules {
  words: number
  separator: string
  capitalize: boolean
  /** A digit after one random word. */
  number: boolean
}
/** A password and its entropy in bits, for an attacker who knows the rules. */
export interface Password {
  value: string
  bits: number
}
export const passwords = (rules: PasswordRules, count: number) => post<{ passwords: Password[] }>('password', { rules, count })
export const passphrases = (phrase: PassphraseRules, count: number) => post<{ passwords: Password[] }>('password', { op: 'passphrase', phrase, count })

/** Version 4 (random), 7 (time-ordered), or 5 and 3 (from a namespace and
 *  name: the same UUID every time, so one comes back). */
export interface UUIDOptions {
  version: 4 | 7 | 5 | 3
  /** dns, url, oid, x500, or a UUID. */
  namespace?: string
  name?: string
}
export const uuids = (count: number, uuid?: UUIDOptions) => post<{ uuids: string[] }>('uuid', { count, uuid })

export interface UUIDInfo {
  uuid: string
  version: number
  kind: string
  variant: string
  /** When a v1, v6 or v7 UUID was made (RFC 3339, UTC). */
  time?: string
  nil?: boolean
  max?: boolean
  /** Set when a ULID was read; uuid is then its 128 bits as a UUID. */
  ulid?: string
}
export const parseUUID = (input: string) => post<UUIDInfo>('uuid', { op: 'parse', input })

/** A checksum to compare with, from a pasted value or a checksums file. */
export interface Expected {
  hex: string
  algorithm: string
  /** The file a checksums line names. */
  name?: string
  line: number
}
export const parseExpected = (input: string) => post<{ expected: Expected[] }>('expected', { input })

/** Every digest of a file sent as is (streamed, never kept). */
export const checksumFile = (file: Blob, signal?: AbortSignal) => postRaw<{ digests: Digest[]; size: number }>('/api/v2/checksum', file, signal)
/** Every digest of a file in the open folder. */
export const checksumPath = (path: string) => request<{ digests: Digest[]; size: number }>('GET', '/api/v2/checksum?path=' + encodeURIComponent(path))

/** Placeholder git commit ids: 40 hex (sha1), 7 (short) or 64 (sha256). */
export type SHAForm = 'sha1' | 'short' | 'sha256'
export const commitSHAs = (form: SHAForm, count: number) => post<{ shas: string[] }>('sha', { form, count })

export const htpasswd = (user: string, password: string, cost: number) =>
  post<{ line: string }>('htpasswd', { op: 'make', user, password, cost })
export const htpasswdCheck = (line: string, password: string) =>
  post<{ match: boolean }>('htpasswd', { op: 'check', line, password })
