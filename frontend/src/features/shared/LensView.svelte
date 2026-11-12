<script lang="ts">
  import Badge from '../../lib/components/Badge.svelte'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import type { Component, Snippet } from 'svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { editorNav, openSessions } from '../editor/active.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'

  // The frame of a lens view of one file (Pipeline, Compose, Ansible):
  // a header, edits in open tabs passed on as what-if overrides, a
  // counter that bumps when files change on disk, and opening a place
  // in the editor.
  export interface LensContext {
    overrides: Record<string, string>
    reload: number
    open: (s: { file?: string; line: number }) => void
    status: (text: string) => void
  }

  interface Props {
    path: string
    active: boolean
    icon: Component
    title: string
    children: Snippet<[LensContext]>
  }

  let { path, active, icon: Icon, title, children }: Props = $props()

  const overrides = $derived.by(() => {
    const out: Record<string, string> = {}
    for (const [p, s] of openSessions) if (s.dirty) out[p] = s.buffer
    return out
  })
  const reload = $derived.by(() => {
    let v = 0
    for (const [, n] of ws.versions) v += n
    return v
  })
  let status = $state('')

  function open(s: { file?: string; line: number }) {
    const file = s.file || path
    layout.openFile(file)
    editorNav.request(file, s.line, 1)
  }
</script>

<div class="view" class:inactive={!active}>
  <PanelHeader icon={Icon} {title}>
    {#snippet meta()}
      <span role="status">{status}</span>
      {#if Object.keys(overrides).length}<Badge tone="warn">What-if</Badge>{/if}
    {/snippet}
  </PanelHeader>
  <div class="body">
    {@render children({ overrides, reload, open, status: (t) => (status = t) })}
  </div>
</div>

<style>
  .view {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .body {
    flex: 1;
    min-height: 0;
  }
</style>
