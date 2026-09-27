import { request } from './client'

export interface VersionInfo {
  version: string
  commit?: string
  date?: string
  dirty?: boolean
}

export function getVersion(): Promise<VersionInfo> {
  return request<VersionInfo>('GET', '/api/version')
}
