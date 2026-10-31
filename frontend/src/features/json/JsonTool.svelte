<script lang="ts">
  import { untrack } from 'svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import WorkTabStrip from '../shared/WorkTabStrip.svelte'
  import { workTabShortcuts } from '../shared/worktabs.svelte'
  import JsonDocument from './JsonDocument.svelte'
  import { JsonDocTab, jsonTabs } from './tabs.svelte'

  // JSON & YAML (3.7.0): one document pasted once and looked at every
  // way, plus a diff of two; several documents open at once in work tabs
  // (decision 98), each remembering the sub-tab it was left on.
  let { active }: { active: boolean } = $props()

  const add = () => jsonTabs.add()
  workTabShortcuts(() => active, jsonTabs, add)

  // The sub-tab shown is the tool's (layout); a document keeps its own:
  // shown again when its tab is, saved when another is picked.
  let shown: JsonDocTab | null = null
  $effect(() => {
    const cur = jsonTabs.current
    const sub = layout.toolTab('json')
    untrack(() => {
      if (cur !== shown) {
        shown = cur
        if (cur?.subtab && cur.subtab !== sub) layout.setToolTab('json', cur.subtab)
      } else if (cur && sub) cur.subtab = sub
    })
  })

  // JSONC or JSON5 handed over by smart paste: into this document when
  // it is empty, else a new one; shown formatted.
  $effect(() => {
    const v = handoff.take('json', 'format')
    if (v === null) return
    untrack(() => {
      const cur = jsonTabs.current
      const t = cur && !cur.doc.hasText ? cur : jsonTabs.add()
      t.doc.input = v
      t.subtab = 'format'
      layout.setToolTab('json', 'format')
    })
  })
</script>

{#snippet strip()}
  <WorkTabStrip tabs={jsonTabs} title={(t) => t.title} label="Open documents" addLabel="New document" onadd={add} rename={(t, name) => (t.name = name)} />
{/snippet}

{#each jsonTabs.list as t (t.key)}
  <div class="tab" class:hidden={t !== jsonTabs.current}>
    <JsonDocument tab={t} active={active && t === jsonTabs.current} {strip} />
  </div>
{/each}

<style>
  .tab {
    height: 100%;
  }
  .tab.hidden {
    display: none;
  }
</style>
