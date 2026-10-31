<script lang="ts">
  import Hammer from '@lucide/svelte/icons/hammer'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { currentTab, type ToolDef } from '../../lib/tools'
  import { layout } from '../../lib/stores/layout.svelte'
  import ToolLayout from './ToolLayout.svelte'
  import ToolTabs from './ToolTabs.svelte'
  import { toolStatus } from './tool.svelte'

  // A sub-tab whose tool is still being built in this release: the tool
  // frame and its tabs work, the page says what is coming.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const tab = $derived(tool.tabs?.find((t) => t.id === currentTab(tool, layout.toolTab(tool.id))))

  toolStatus(() => active, () => ({ text: 'Not built yet' }))
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <EmptyState icon={tab?.icon ?? Hammer} title="{tab?.title ?? tool.title} is on its way" description="{tab?.description ?? ''} It arrives in a later step of this release." />
  {/snippet}
</ToolLayout>
