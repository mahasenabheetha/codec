// Where a tool's output can go (3.10.2): the tools that take text
// handed to them (handoff.take), in the order they are most likely
// wanted. The hand-off is in memory and taken once (decision 19).
import type { MenuEntry } from '../../lib/components/ContextMenu.svelte'
import { handoff } from '../../lib/stores/handoff.svelte'
import { layout } from '../../lib/stores/layout.svelte'
import { router } from '../../lib/stores/router.svelte'
import { toolById } from '../../lib/tools'

const targets: [tool: string, tab: string][] = [
  ['json', 'format'],
  ['text', 'sanitize'],
  ['text', 'table'],
  ['logs', 'viewer'],
  ['encode', 'url'],
  ['commands', 'explain'],
  ['time', 'timestamp'],
  ['time', 'cron'],
  ['network', 'http'],
  ['network', 'cidr'],
  ['network', 'ip'],
  ['encode', 'uuid'],
  ['colour', ''],
]

/** The menu entries that send `text` on; the tool and sub-tab showing
 *  now is left out. */
export function sendTo(text: () => string): MenuEntry[] {
  const here = router.toolId
  const hereTab = here ? (layout.toolTab(here) ?? '') : ''
  const out: MenuEntry[] = []
  for (const [tool, tab] of targets) {
    const t = toolById(tool)
    if (!t) continue
    const sub = t.tabs?.find((x) => x.id === tab)
    // The same place: nothing to send. A tool without sub-tabs is one place.
    if (tool === here && (!t.tabs || !sub || tab === hereTab)) continue
    out.push({ label: sub ? `${t.title} · ${sub.title}` : t.title, icon: sub?.icon ?? t.icon, run: () => handoff.send(tool, tab, text()) })
  }
  return out
}
