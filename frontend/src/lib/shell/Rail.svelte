<script lang="ts">
  import House from '@lucide/svelte/icons/house'
  import Keyboard from '@lucide/svelte/icons/keyboard'
  import PanelLeftClose from '@lucide/svelte/icons/panel-left-close'
  import PanelLeftOpen from '@lucide/svelte/icons/panel-left-open'
  import type { Component } from 'svelte'
  import Tooltip from '../components/Tooltip.svelte'
  import { layout } from '../stores/layout.svelte'
  import { router } from '../stores/router.svelte'
  import { toolGroups } from '../tools'
  import { chain } from '../utils/events'

  const groups = toolGroups()
  const collapsed = $derived(layout.railCollapsed)
</script>

{#snippet item(icon: Component, label: string, selected: boolean, onclick: () => void, shortcut?: string)}
  {@const Icon = icon}
  <Tooltip text={label} {shortcut} side="right" off={!collapsed}>
    {#snippet children(props)}
      <button
        {...props}
        type="button"
        class="item"
        class:selected
        aria-current={selected ? 'page' : undefined}
        aria-label={label}
        onclick={chain(props.onclick, onclick)}
      >
        <Icon size={17} strokeWidth={1.75} />
        {#if !collapsed}<span class="label">{label}</span>{/if}
      </button>
    {/snippet}
  </Tooltip>
{/snippet}

<nav class="rail" class:collapsed aria-label="Tools">
  <div class="brand">
    <img src="/favicon.svg" alt="" width="22" height="22" />
    {#if !collapsed}<span>codec</span>{/if}
  </div>

  <div class="items">
    {@render item(House, 'Home', router.path === '/', () => router.go('/'))}

    {#each groups as g (g.group)}
      {#if collapsed}
        <div class="sep" role="separator"></div>
      {:else}
        <div class="group">{g.group}</div>
      {/if}
      {#each g.tools as t (t.id)}
        {@render item(t.icon, t.title, router.toolId === t.id, () => layout.openTool(t.id))}
      {/each}
    {/each}
  </div>

  <div class="bottom">
    {@render item(Keyboard, 'Keyboard shortcuts', false, () => (layout.shortcutsOpen = true), '?')}
    {@render item(
      collapsed ? PanelLeftOpen : PanelLeftClose,
      collapsed ? 'Expand sidebar' : 'Collapse sidebar',
      false,
      () => layout.toggleRail(),
      'Mod+B',
    )}
  </div>
</nav>

<style>
  .rail {
    width: var(--rail-expanded);
    height: 100%;
    display: flex;
    flex-direction: column;
    padding: var(--s-2);
    background: var(--bg-0);
    border-right: 1px solid var(--border);
    transition: width 160ms var(--ease);
    overflow: hidden;
  }
  .rail.collapsed {
    width: var(--rail-collapsed);
    padding: var(--s-2) var(--s-1);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 36px;
    padding: 0 var(--s-2);
    margin-bottom: var(--s-2);
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.02em;
  }
  .collapsed .brand {
    justify-content: center;
    padding: 0;
  }
  .items {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow-y: auto;
    overflow-x: hidden;
  }
  .group {
    margin: var(--s-4) 0 var(--s-1);
    padding: 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
    white-space: nowrap;
  }
  .sep {
    height: 1px;
    margin: var(--s-2) var(--s-2);
    background: var(--border);
  }
  .item {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    width: 100%;
    height: 32px;
    padding: 0 var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
    white-space: nowrap;
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  .collapsed .item {
    justify-content: center;
    padding: 0;
  }
  .item:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .item.selected {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .item.selected :global(svg) {
    color: var(--accent);
  }
  .bottom {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-top: var(--s-2);
    border-top: 1px solid var(--border);
  }
</style>
