<script lang="ts">
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { currentTab, toolById } from '../../lib/tools'
  import ToolLayout from './ToolLayout.svelte'
  import ToolTabs from './ToolTabs.svelte'

  // Stands in for a Utilities tool until its task builds it (2.3.0).
  let { id }: { id: string } = $props()

  const tool = $derived(toolById(id)!)
  const tab = $derived(tool.tabs?.find((t) => t.id === currentTab(tool, layout.toolTab(tool.id))) ?? tool)
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    {#if tool.tabs}<ToolTabs {tool} />{/if}
  {/snippet}
  {#snippet input()}
    <EmptyState icon={tab.icon} title={tab.title} description={tab.description} />
  {/snippet}
  {#snippet output()}
    <EmptyState icon={tab.icon} title="Not built yet" />
  {/snippet}
</ToolLayout>
