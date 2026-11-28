import { request } from './client'

export interface VersionInfo {
  version: string
  commit?: string
  date?: string
  dirty?: boolean
}

let asked: Promise<VersionInfo> | undefined

/** The running codec's version, asked once and shared (the rail and the
 *  status bar both show it); asked again after a failure. */
export function getVersion(): Promise<VersionInfo> {
  asked ??= request<VersionInfo>('GET', '/api/version').catch((e) => {
    asked = undefined
    throw e
  })
  return asked
}

/** "v3.11.0" as "3.11", for the rail; a build without a release number
 *  ("dev") as it is. */
export function shortVersion(v: string): string {
  return /^v?(\d+\.\d+)/.exec(v)?.[1] ?? v
}
