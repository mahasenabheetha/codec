// Desktop window endpoints (internal/web/desktop.go). They exist only
// when the page runs in the desktop app; use lib/platform.ts, which
// checks that first.
import { request } from './client'

export function setWindowTheme(theme: 'dark' | 'light'): Promise<void> {
  return request('POST', '/api/v2/desktop/theme', { theme })
}
