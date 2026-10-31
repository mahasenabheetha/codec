<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import type { ToolDef } from '../../lib/tools'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'

  // A Wellness sub-tab whose task hasn't landed yet (3.6.0 T3 to T5).
  interface Props {
    tool: ToolDef
    icon: Component
    title: string
    description: string
    children?: Snippet
  }

  let { tool, icon, title, description, children }: Props = $props()
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <div class="planned">
      <EmptyState {icon} {title} {description} />
    </div>
    {#if children}{@render children()}{/if}
  {/snippet}
</ToolLayout>

<style>
  .planned {
    min-height: 220px;
  }
</style>
