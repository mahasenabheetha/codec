<script lang="ts">
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
      {#if Object.keys(overrides).length}<Badge tone="warn">What-if</Badge>{/if}
    {/snippet}
  </PanelHeader>
  <div class="body">
    <ArgoPanel source={{ kind: 'file', path, overrides }} {reload} onopen={open} onstatus={(t) => (status = t)} />
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
