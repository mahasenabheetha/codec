// How a tool lays its input and output out: side by side when there is
// room (auto), or always one over the other. A display preference per
// tool, persisted; the sizes themselves are SplitPane's.
import { persisted } from '../../lib/stores/persist.svelte'

export type PaneLayout = 'auto' | 'column'

const prefs = persisted<Record<string, PaneLayout>>('paneLayout', {})

export const paneLayout = {
  of(id: string): PaneLayout {
    return prefs.value[id] ?? 'auto'
  },
  set(id: string, v: PaneLayout) {
    prefs.value = { ...prefs.value, [id]: v }
  },
}
