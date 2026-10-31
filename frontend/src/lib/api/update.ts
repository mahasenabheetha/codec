// The release check (internal/web/update_api.go, decision 85).
import { request } from './client'

export interface Release {
  version: string
  url: string
  published: string
  prerelease?: boolean
}

export interface UpdateInfo {
  current: string
  latest?: Release
  newer: boolean
  checked?: string
  /** "off" (the daily check is off) or "dev" (a development build). */
  skipped?: string
  enabled: boolean
  error?: string
}

/** The day's answer; `force` asks GitHub now ("Check now"). */
export function getUpdate(force = false): Promise<UpdateInfo> {
  return request('GET', '/api/v2/update' + (force ? '?force=1' : ''))
}

export function setUpdateCheck(check: boolean): Promise<{ check: boolean }> {
  return request('POST', '/api/v2/update/settings', { check })
}
