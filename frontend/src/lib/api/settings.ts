import { request } from './client'
import type { HelmProfile } from './helm'

export type FolderID = 'settings' | 'templates' | 'schemas' | 'sample'

export interface SettingsFolder {
  id: FolderID
  path: string // '' = none on this system
  exists: boolean
}

export interface ChartProfiles {
  chart: string // absolute chart folder
  exists: boolean
  profiles: Record<string, HelmProfile>
  active?: string
}

export interface SettingsInfo {
  folders: SettingsFolder[]
  helm: ChartProfiles[]
  recent: string[]
}

export const getSettings = () => request<SettingsInfo>('GET', '/api/v2/settings')

/** Remove a saved Helm profile (no profile = all of the chart's), or
 *  the recent folders list. */
export const forget = (what: { helm?: { chart: string; profile?: string }; recent?: boolean }) =>
  request<SettingsInfo>('POST', '/api/v2/settings/forget', what)
