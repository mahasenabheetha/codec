<script lang="ts">
  import House from '@lucide/svelte/icons/house'
  import Keyboard from '@lucide/svelte/icons/keyboard'
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal'
  import PanelLeft from '@lucide/svelte/icons/panel-left'
  import Pin from '@lucide/svelte/icons/pin'
  import PinOff from '@lucide/svelte/icons/pin-off'
  import type { Component } from 'svelte'
  import Badge from '../components/Badge.svelte'
  import BrandMark from '../components/BrandMark.svelte'
  import ContextMenu, { type MenuEntry } from '../components/ContextMenu.svelte'
  import Tooltip from '../components/Tooltip.svelte'
  import { getVersion, shortVersion } from '../api/version'
  import { usage } from '../stores/usage.svelte'
  import { workspace } from '../../features/workspace/workspace.svelte'
  import { finder } from '../../features/search/search.svelte'
  import { lint } from '../../features/lint/lint.svelte'
  import { layout } from '../stores/layout.svelte'
  import { compareRoute, settingsRoute, newRoute, problemsRoute, queryRoute, router } from '../stores/router.svelte'
  import { entryByKey, hueVar, toolGroups, viewHue, viewIcon, type Hue, type ToolEntry } from '../tools'
  import { chain } from '../utils/events'

  const groups = toolGroups()
  const collapsed = $derived(layout.railCollapsed)

  let version = $state('')
  getVersion()
    .then((v) => (version = shortVersion(v.version)))
    .catch(() => {})

  // Pinned tools and sub-tabs, first on the rail (3.10.1): pin one from
  // its menu here (right-click, or Shift+F10) or its star on Home.
  const pins = $derived(usage.pinned.map(entryByKey).filter((e): e is ToolEntry => !!e))

  function menuFor(e: MouseEvent): MenuEntry[] {
    const key = (e.target as HTMLElement).closest<HTMLElement>('[data-pin]')?.dataset.pin
    if (!key) return []
    const on = usage.isPinned(key)
    return [{ label: on ? 'Unpin from the top' : 'Pin to the top', icon: on ? PinOff : Pin, run: () => usage.togglePin(key) }]
  }

  interface Item {
    icon: Component
    label: string
    selected?: boolean
    onclick: () => void
    shortcut?: string
    hue?: Hue
    pin?: string
    /** What it holds, after the label (Problems found). */
    count?: number
  }
</script>

{#snippet item({ icon: Icon, label, selected = false, onclick, shortcut, hue, pin, count }: Item)}
  <Tooltip text={label} {shortcut} side="right" off={!collapsed}>
    {#snippet children(props)}
      <button
        {...props}
        type="button"
        class="item"
        class:selected
        class:hued={!!hue}
        style:--tool={hue && hueVar(hue)}
        aria-current={selected ? 'page' : undefined}
        aria-label={count ? `${label}, ${count}` : label}
        data-pin={pin}
        onclick={chain(props.onclick, onclick)}
      >
        <span class="tile" aria-hidden="true"><Icon size={14} /></span>
        {#if !collapsed}
          <span class="label">{label}</span>
          {#if count}<span class="count">{count}</span>{/if}
        {/if}
      </button>
    {/snippet}
  </Tooltip>
{/snippet}

{#snippet group(label: string)}
  {#if collapsed}
    <div class="group" role="separator"></div>
  {:else}
    <div class="group"><span>{label}</span></div>
  {/if}
{/snippet}

{#snippet bare(icon: Component, label: string, selected: boolean, onclick: () => void, shortcut: string)}
  {@const Icon = icon}
  <Tooltip text={label} {shortcut} side={collapsed ? 'right' : 'top'}>
    {#snippet children(props)}
      <button
        {...props}
        type="button"
        class="bare"
        class:selected
        aria-current={selected ? 'page' : undefined}
        aria-label={label}
        onclick={chain(props.onclick, onclick)}
      >
        <Icon size={16} />
      </button>
    {/snippet}
  </Tooltip>
{/snippet}

<nav class="rail" class:collapsed aria-label="Tools">
  <div class="brand">
    <BrandMark />
    {#if !collapsed}
      <span class="name">codec</span>
      {#if version}<span class="version" title="The codec version"><Badge mono>{version}</Badge></span>{/if}
    {/if}
  </div>

  <div class="items">
   <ContextMenu label="Tool" items={menuFor}>
    {@render item({ icon: House, label: 'Home', selected: router.path === '/', onclick: () => router.go('/') })}

    {#if pins.length}
      {@render group('Pinned')}
      {#each pins as p (p.key)}
        {@render item({
          icon: p.icon,
          label: p.title,
          selected: router.toolId === p.tool && (!p.tab || layout.toolTab(p.tool) === p.tab),
          onclick: () => layout.openTool(p.tool, p.tab),
          hue: p.hue,
          pin: p.key,
        })}
      {/each}
    {/if}

    {@render group('Workspace')}
    {@render item({
      icon: viewIcon.explorer,
      label: 'Explorer',
      selected: layout.explorerOpen && layout.sidebarView === 'explorer',
      onclick: () => layout.toggleExplorer(),
      shortcut: 'Mod+Shift+E',
      hue: viewHue.explorer,
    })}
    {@render item({
      icon: viewIcon.search,
      label: 'Search',
      selected: layout.explorerOpen && layout.sidebarView === 'search',
      onclick: () => {
        layout.toggleSearch()
        finder.focusTick++
      },
      shortcut: 'Mod+Shift+F',
      hue: viewHue.search,
    })}
    {@render item({ icon: viewIcon.open, label: 'Open folder…', onclick: () => workspace.chooseFolder(), shortcut: 'Mod+O', hue: viewHue.open })}
    {@render item({
      icon: viewIcon.problems,
      label: 'Problems',
      selected: router.path === problemsRoute,
      onclick: () => layout.open(problemsRoute),
      hue: viewHue.problems,
      count: lint.found,
    })}
    {@render item({ icon: viewIcon.compare, label: 'Compare', selected: router.path === compareRoute, onclick: () => layout.open(compareRoute), hue: viewHue.compare })}
    {@render item({ icon: viewIcon.query, label: 'Query', selected: router.path === queryRoute, onclick: () => layout.open(queryRoute), hue: viewHue.query })}
    {@render item({ icon: viewIcon.new, label: 'New', selected: router.path === newRoute, onclick: () => layout.open(newRoute), hue: viewHue.new })}

    {#each groups as g (g.group)}
      {@render group(g.group)}
      {#each g.tools as t (t.id)}
        {@render item({ icon: t.icon, label: t.title, selected: router.toolId === t.id, onclick: () => layout.openTool(t.id), hue: t.hue, pin: t.id })}
      {/each}
    {/each}
   </ContextMenu>
  </div>

  <!-- Three icons in a row while the rail is open (a column when it is
       collapsed): the list above keeps the room. -->
  <div class="bottom">
    {@render bare(SlidersHorizontal, 'Settings', router.path === settingsRoute, () => layout.open(settingsRoute), 'Mod+,')}
    {@render bare(Keyboard, 'Keyboard shortcuts', false, () => (layout.shortcutsOpen = true), '?')}
    {@render bare(PanelLeft, collapsed ? 'Expand sidebar' : 'Collapse sidebar', false, () => layout.toggleRail(), 'Mod+B')}
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
  }
  .collapsed .brand {
    justify-content: center;
    padding: 0;
  }
  .name {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.01em;
  }
  .version {
    margin-left: auto;
  }
  .items {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow-y: auto;
    overflow-x: hidden;
    /* The last entries fade out where the list scrolls on, so a cut-off
       row reads as "more below", not as a broken one. */
    padding-bottom: var(--s-3);
    mask-image: linear-gradient(to bottom, #000 calc(100% - var(--s-4)), transparent);
  }
  /* A group's name with a line to the edge; only the line while the rail
     is collapsed. */
  .group {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-2) var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    white-space: nowrap;
  }
  .group::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--border);
  }
  .collapsed .group {
    padding: var(--s-2) var(--s-1);
  }
  .item {
    flex: none;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    height: calc(var(--control-h) - 2px);
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
    height: calc(var(--control-h) + 2px);
    padding: 0;
  }
  .item:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  /* The item's icon on a tile in its tool's or view's colour (the accent
     for Home), as Tile draws it; solid for the one that is open. */
  .tile {
    flex: none;
    display: grid;
    place-items: center;
    width: calc(var(--control-h) - 10px);
    height: calc(var(--control-h) - 10px);
    color: color-mix(in oklab, var(--tool-accent, var(--accent)) var(--tint-tool), var(--fg-1));
    background: color-mix(in oklab, var(--tool-accent-soft, var(--accent-soft)) var(--tint-tool), var(--bg-3));
    border-radius: var(--r-sm);
  }
  .item.selected {
    color: var(--fg-0);
    font-weight: var(--fw-semibold);
    background: color-mix(in oklab, var(--tool-accent-soft, var(--accent-soft)) var(--tint-fill), var(--bg-3));
  }
  .item.selected .tile {
    color: var(--accent-fg);
    background: var(--tool-accent, var(--accent));
  }
  .label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .count {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    font-weight: var(--fw-regular);
    color: var(--fg-2);
  }
  .bottom {
    display: flex;
    gap: var(--s-1);
    padding-top: var(--s-2);
    border-top: 1px solid var(--border);
  }
  .collapsed .bottom {
    flex-direction: column;
  }
  .bare {
    flex: 1;
    display: grid;
    place-items: center;
    height: calc(var(--control-h) - 2px);
    padding: 0;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  .bare:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .bare.selected {
    color: var(--fg-0);
    background: var(--bg-3);
  }
</style>
