<script lang="ts">
  import type { Component } from 'svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { currentTab, type ToolDef } from '../../lib/tools'

  // A tool made of sub-tabs, one component each. A sub-tab is mounted
  // when first shown and then kept, so switching back finds it as it was.
  interface Props {
    tool: ToolDef
    active: boolean
    tabs: Record<string, Component<{ tool: ToolDef; active: boolean }>>
  }

  let { tool, active, tabs }: Props = $props()

  const tab = $derived(currentTab(tool, layout.toolTab(tool.id)))
  const seen = $state<Record<string, boolean>>({})
  $effect(() => {
    seen[tab] = true
  })
</script>

{#each Object.entries(tabs) as [id, Tab] (id)}
  {#if seen[id] || id === tab}
    <div class="tab" data-subtab={id} hidden={id !== tab}>
      <Tab {tool} active={active && id === tab} />
    </div>
  {/if}
{/each}

<style>
  .tab {
    height: 100%;
  }
  .tab[hidden] {
    display: none;
  }
</style>
