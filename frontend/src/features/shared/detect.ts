import type { Language } from '../../lib/components/CodeView.svelte'

/** Pick an editor language for display only — the backend decides what
 *  content *is*; this just chooses highlighting for its output. */
export function displayLanguage(text: string): Language {
  const s = text.trimStart()
  if ((s.startsWith('{') || s.startsWith('[')) && text.length < 5_000_000) {
    try {
      JSON.parse(text)
      return 'json'
    } catch {
      /* not JSON */
    }
  }
  return 'text'
}

/** "1,234 chars · 12 lines" for pane headers. */
export function sizeLabel(text: string): string {
  if (!text) return ''
  const lines = text.split('\n').length
  return `${text.length.toLocaleString()} chars · ${lines.toLocaleString()} line${lines === 1 ? '' : 's'}`
}
