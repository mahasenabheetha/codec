<script lang="ts">
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { toolCommands } from './tool.svelte'
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

  // The files with what-if edits this view is drawn from, and the way
  // back to what is on disk (3.10.2: every lens says it the same way).
  const edited = $derived(Object.keys(overrides))
  function resetEdits() {
    for (const [, s] of openSessions) if (s.dirty) s.reset()
  }
  toolCommands(
    () => active,
    () => [
      { id: 'lens.open', title: 'Open this view\'s file in the editor', group: 'View', run: () => layout.openFile(path) },
      { id: 'lens.reset', title: 'Reset the what-if edits', group: 'View', run: resetEdits },
    ],
  )

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
      {#if edited.length}
        <span class="whatif" title={'What-if edits in open tabs, never saved:\n' + edited.join('\n')}>
          <Badge tone="warn">What-if · {edited.length}</Badge>
          <IconButton icon={RotateCcw} label="Reset the what-if edits ({edited.length} {edited.length === 1 ? 'file' : 'files'})" size="sm" onclick={resetEdits} />
        </span>
      {/if}
    {/snippet}
  </PanelHeader>
  <div class="body">
    {@render children({ overrides, reload, open, status: (t) => (status = t) })}
  </div>
</div>

<style>
  .whatif {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }
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
