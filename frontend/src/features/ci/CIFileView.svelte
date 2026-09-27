<script lang="ts">
  import GitBranch from '@lucide/svelte/icons/git-branch'
  import LensView from '../shared/LensView.svelte'
  import CIPanel from './CIPanel.svelte'

  // The pipeline view of one CI file (includes, templates and reusable
  // workflows followed across the folder).
  let { path, active }: { path: string; active: boolean } = $props()
</script>

<LensView {path} {active} icon={GitBranch} title="Pipeline · {path.split('/').pop()}">
  {#snippet children(ctx)}
    <CIPanel {path} overrides={ctx.overrides} reload={ctx.reload} onopen={ctx.open} onstatus={ctx.status} />
  {/snippet}
</LensView>
