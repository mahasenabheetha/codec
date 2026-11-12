<script lang="ts">
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { toolCommands } from '../shared/tool.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import Workflow from '@lucide/svelte/icons/workflow'
  import type { ArgoSource } from '../../lib/api/argo'
  import { layout } from '../../lib/stores/layout.svelte'
  import { editorNav, openSessions } from '../editor/active.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import ArgoPanel from './ArgoPanel.svelte'

  // The Argo view of one file: its workflows (templateRefs resolved
  // across the folder) and applications. Edits in open tabs apply as
  // what-if; files changing on disk re-analyze.
  interface Props {
    path: string
    active: boolean
  }

  let { path, active }: Props = $props()

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

  function open(s: ArgoSource) {
    const file = s.file ?? path
    layout.openFile(file)
    editorNav.request(file, s.line, 1)
  }
</script>

<div class="view" class:inactive={!active}>
  <PanelHeader icon={Workflow} title="Argo · {path.split('/').pop()}">
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
    <ArgoPanel source={{ kind: 'file', path, overrides }} {reload} onopen={open} onstatus={(t) => (status = t)} />
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
