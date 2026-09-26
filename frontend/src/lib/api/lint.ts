import { request } from './client'
import type { Diagnostic } from './yaml'

export type Level = 'error' | 'warning' | 'info' | 'off'

export interface LintRule {
  id: string
  group: 'style' | 'kubernetes' | 'deprecation' | 'schema'
  title: string
  default: Level
  why: string
}

/** The user's lint choices (zero values mean defaults). */
export interface LintSettings {
  rules?: Record<string, Level>
  lineLength?: number
  k8sVersion?: string
  noSchemas?: boolean
  offline?: boolean
  schemaDir?: string
}

export interface LintSettingsResponse {
  settings: LintSettings
  rules: LintRule[]
  k8sVersions: string[]
  defaultK8sVersion: string
}

export interface LintedFile {
  path: string
  type: string
  diagnostics: Diagnostic[]
}

export const getLintSettings = () => request<LintSettingsResponse>('GET', '/api/v2/lint/settings')

export const saveLintSettings = (settings: LintSettings) =>
  request<{ ok: boolean }>('POST', '/api/v2/lint/settings', { settings })

export const lintWorkspace = (signal?: AbortSignal) =>
  request<{ files: LintedFile[]; checked: number; durationMs: number }>('POST', '/api/v2/lint/workspace', {}, signal)
