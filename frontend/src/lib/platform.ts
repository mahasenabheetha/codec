// Where the page runs: a browser tab (`codec serve`) or the desktop
// window (cmd/codec-desktop), which the server marks with a meta tag.
// Desktop-only features go through here, each with a browser fallback,
// so the web UI never depends on them.
import { pickFolder as pickFolderAPI, setWindowTheme } from './api/desktop'

export const desktop = document.querySelector('meta[name="codec-desktop"]') !== null

/** Match the desktop window's frame to the page's theme. */
export function windowTheme(theme: 'dark' | 'light') {
  if (desktop) setWindowTheme(theme).catch(() => {}) // cosmetic; never worth an error
}

/** The system folder dialog in the desktop app: the chosen path, '' when
 *  cancelled, or null in a browser (use codec's own folder browser). */
export function pickFolder(title: string, start = ''): Promise<string> | null {
  return desktop ? pickFolderAPI(title, start) : null
}
