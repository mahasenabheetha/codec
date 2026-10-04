// Desktop window endpoints (internal/web/desktop.go). They exist only
// when the page runs in the desktop app; use lib/platform.ts, which
// checks that first.
import { request } from './client'

export function setWindowTheme(theme: 'dark' | 'light'): Promise<void> {
  return request('POST', '/api/v2/desktop/theme', { theme })
}

/** The system folder dialog; resolves to '' when cancelled. */
export async function pickFolder(title: string, start: string): Promise<string> {
  return (await request<{ path: string }>('POST', '/api/v2/desktop/pick-folder', { title, start })).path
}

export interface DesktopSettings {
  startAtLogin: boolean
  keepInTray: boolean
}

export function getDesktopSettings(): Promise<DesktopSettings> {
  return request('GET', '/api/v2/desktop/settings')
}

export function setDesktopSettings(st: DesktopSettings): Promise<DesktopSettings> {
  return request('POST', '/api/v2/desktop/settings', st)
}
