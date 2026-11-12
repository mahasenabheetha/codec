// Settings from the palette (3.10.1): "Settings: Theme" opens the page
// on that row. The entries name the section and, for one setting, its
// row title as SettingRow shows it; SettingsView scrolls there and
// marks the row for a moment.
export interface SettingEntry {
  title: string
  section: string
  /** The row's title; left out for a whole section. */
  row?: string
  keywords: string[]
}

export const settingEntries: SettingEntry[] = [
  { title: 'Theme', section: 'general', row: 'Theme', keywords: ['dark', 'light', 'dusk', 'midnight', 'paper', 'mist', 'contrast', 'colour', 'color'] },
  { title: 'Tool colours', section: 'general', row: 'Tool colours', keywords: ['hue', 'plain', 'accent', 'color'] },
  { title: 'Interface size', section: 'general', row: 'Interface size', keywords: ['compact', 'comfortable', 'large', 'font', 'text size', 'zoom', 'density'] },
  { title: 'File icons', section: 'general', row: 'File icons', keywords: ['material', 'simple'] },
  { title: 'Open files in', section: 'general', row: 'Open files in', keywords: ['editor', 'vs code', 'cursor'] },
  { title: 'Outline and problems panel', section: 'general', row: 'Outline and problems panel', keywords: ['side panel', 'lens'] },
  { title: 'Open Markdown as', section: 'general', row: 'Open Markdown as', keywords: ['preview', 'source', 'md'] },
  { title: 'Check for updates', section: 'general', row: 'Check for updates', keywords: ['release', 'version', 'upgrade'] },
  { title: 'Kubernetes target version', section: 'kubernetes', row: 'Target version', keywords: ['k8s', 'api', 'removed'] },
  { title: 'Schemas', section: 'schemas', keywords: ['validate', 'offline', 'custom schema folder', 'crd'] },
  { title: 'Lint rules', section: 'rules-style', keywords: ['level', 'error', 'warning', 'off', 'line length'] },
  { title: 'Helm profiles', section: 'helm', keywords: ['saved', 'forget', 'values'] },
  { title: 'Compare: ignored paths', section: 'compare', keywords: ['ignore', 'diff'] },
  { title: 'Wellness: sounds and notifications', section: 'wellness', keywords: ['sound', 'chime', 'volume', 'mute', 'notification', 'focus', 'break'] },
  { title: 'Folders', section: 'folders', keywords: ['settings file', 'cache', 'starters', 'templates', 'recent'] },
]

class Goto {
  /** What to show, and a counter so the same entry can be asked for twice. */
  target = $state<{ section: string; row?: string; tick: number } | null>(null)

  to(e: SettingEntry) {
    this.target = { section: e.section, row: e.row, tick: (this.target?.tick ?? 0) + 1 }
  }
}

export const settingsGoto = new Goto()
