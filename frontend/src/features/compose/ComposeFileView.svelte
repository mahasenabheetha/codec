<script lang="ts">
  import Container from '@lucide/svelte/icons/container'
  import LensView from '../shared/LensView.svelte'
  import ComposePanel from './ComposePanel.svelte'

  // The Compose view of one file: its project (the file, its override
  // or the layers chosen), with what-if variables.
  let { path, active }: { path: string; active: boolean } = $props()
</script>

<LensView {path} {active} icon={Container} title="Compose · {path.split('/').pop()}">
  {#snippet children(ctx)}
    <ComposePanel {path} overrides={ctx.overrides} reload={ctx.reload} onopen={ctx.open} onstatus={ctx.status} />
  {/snippet}
</LensView>
