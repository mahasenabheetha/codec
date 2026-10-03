// The editor in the active tab, for app-wide UI (status bar breadcrumb,
// problem counts) and the "open in" preference.

import { SvelteMap } from 'svelte/reactivity'
import { persisted } from '../../lib/stores/persist.svelte'
import type { FileSession } from './session.svelte'

class ActiveEditor {
  session = $state<FileSession | null>(null)
}

export const activeEditor = new ActiveEditor()

export type ExternalEditor = 'vscode' | 'cursor'

/** Which editor "Open in…" launches. A UI preference, so browser storage. */
export const openIn = persisted<ExternalEditor>('openIn', 'vscode')

export const editorNames: Record<ExternalEditor, string> = { vscode: 'VS Code', cursor: 'Cursor' }

/** vscode://file/C:/repo/a.yaml:12:5 style link for a file and position. */
export function editorLink(editor: ExternalEditor, root: string, path: string, line: number, col: number): string {
  const abs = (root.replace(/[\\/]+$/, '') + '/' + path).replace(/\\/g, '/')
  const encoded = abs.split('/').map(encodeURIComponent).join('/').replace(/^([A-Za-z])%3A/, '$1:')
  return `${editor}://file/${encoded.startsWith('/') ? encoded.slice(1) : encoded}:${line}:${col}`
}

/** Whether file tabs show the outline/problems panel. One store for all
 *  tabs: persisted() values are per call, not shared by key. */
export const lensOpen = persisted('lensOpen', true)

/** Every open file editor by path. The Helm view reads their what-if
 *  buffers so a chart renders with unsaved edits. */
export const openSessions = new SvelteMap<string, FileSession>()

/** A request to show a position in a file tab; the tab's editor takes
 *  it once it is ready (e.g. clicking a Helm problem). */
class EditorNav {
  pending = $state<{ path: string; line: number; col: number; endCol?: number } | null>(null)

  /** endCol selects up to that column on the same line (a search hit). */
  request(path: string, line: number, col = 1, endCol?: number) {
    this.pending = { path, line: Math.max(1, line), col: Math.max(1, col), endCol }
  }
}

export const editorNav = new EditorNav()
