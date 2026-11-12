<script lang="ts">
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import FolderTree from '@lucide/svelte/icons/folder-tree'
  import House from '@lucide/svelte/icons/house'
  import Keyboard from '@lucide/svelte/icons/keyboard'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import ListChecks from '@lucide/svelte/icons/list-checks'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import Search from '@lucide/svelte/icons/search'
  import FilePlus from '@lucide/svelte/icons/file-plus'
  import Settings2 from '@lucide/svelte/icons/settings-2'
  import PanelLeftClose from '@lucide/svelte/icons/panel-left-close'
  import PanelLeftOpen from '@lucide/svelte/icons/panel-left-open'
  import Pin from '@lucide/svelte/icons/pin'
  import PinOff from '@lucide/svelte/icons/pin-off'
  import type { Component } from 'svelte'
  import ContextMenu, { type MenuEntry } from '../components/ContextMenu.svelte'
  import Tooltip from '../components/Tooltip.svelte'
  import { usage } from '../stores/usage.svelte'
  import { workspace } from '../../features/workspace/workspace.svelte'
  import { finder } from '../../features/search/search.svelte'
  import { layout } from '../stores/layout.svelte'
  import { compareRoute, settingsRoute, newRoute, problemsRoute, queryRoute, router } from '../stores/router.svelte'
  import { entryByKey, hueVar, toolGroups, viewHue, type Hue, type ToolEntry } from '../tools'
  import { chain } from '../utils/events'

  const groups = toolGroups()
  const collapsed = $derived(layout.railCollapsed)

  // Pinned tools and sub-tabs, first on the rail (3.10.1): pin one from
  // its menu here (right-click, or Shift+F10) or its star on Home.
  const pins = $derived(usage.pinned.map(entryByKey).filter((e): e is ToolEntry => !!e))

  function menuFor(e: MouseEvent): MenuEntry[] {
    const key = (e.target as HTMLElement).closest<HTMLElement>('[data-pin]')?.dataset.pin
    if (!key) return []
    const on = usage.isPinned(key)
    return [{ label: on ? 'Unpin from the top' : 'Pin to the top', icon: on ? PinOff : Pin, run: () => usage.togglePin(key) }]
  }
</script>

{#snippet item(icon: Component, label: string, selected: boolean, onclick: () => void, shortcut?: string, hue?: Hue, pin?: string, bare = false)}
  {@const Icon = icon}
  <Tooltip text={label} {shortcut} side={bare && !collapsed ? 'top' : 'right'} off={!collapsed && !bare}>
    {#snippet children(props)}
      <button
        {...props}
        type="button"
        class="item"
        class:selected
        class:hued={!!hue}
        class:bare
        style:--tool={hue && hueVar(hue)}
        aria-current={selected ? 'page' : undefined}
        aria-label={label}
        data-pin={pin}
        onclick={chain(props.onclick, onclick)}
      >
        <Icon size={17} strokeWidth={1.75} />
        {#if !collapsed && !bare}<span class="label">{label}</span>{/if}
      </button>
    {/snippet}
  </Tooltip>
{/snippet}

<nav class="rail" class:collapsed aria-label="Tools">
  <div class="brand">
    <img src="/icon.svg" alt="" width="22" height="22" />
    {#if !collapsed}<span>codec</span>{/if}
  </div>

  <div class="items">
   <ContextMenu label="Tool" items={menuFor}>
    {@render item(House, 'Home', router.path === '/', () => router.go('/'))}

    {#if pins.length}
      {#if collapsed}
        <div class="sep" role="separator"></div>
      {:else}
        <div class="group">Pinned</div>
      {/if}
      {#each pins as p (p.key)}
        {@render item(p.icon, p.title, router.toolId === p.tool && (!p.tab || layout.toolTab(p.tool) === p.tab), () => layout.openTool(p.tool, p.tab), undefined, p.hue, p.key)}
      {/each}
    {/if}

    {#if collapsed}
      <div class="sep" role="separator"></div>
    {:else}
      <div class="group">Workspace</div>
    {/if}
    {@render item(FolderTree, 'Explorer', layout.explorerOpen && layout.sidebarView === 'explorer', () => layout.toggleExplorer(), 'Mod+Shift+E', viewHue.explorer)}
    {@render item(Search, 'Search', layout.explorerOpen && layout.sidebarView === 'search', () => {
      layout.toggleSearch()
      finder.focusTick++
    }, 'Mod+Shift+F', viewHue.search)}
    {@render item(FolderOpen, 'Open folder…', false, () => workspace.chooseFolder(), 'Mod+O', viewHue.open)}
    {@render item(ListChecks, 'Problems', router.path === problemsRoute, () => layout.open(problemsRoute), undefined, viewHue.problems)}
    {@render item(GitCompare, 'Compare', router.path === compareRoute, () => layout.open(compareRoute), undefined, viewHue.compare)}
    {@render item(TextSearch, 'Query', router.path === queryRoute, () => layout.open(queryRoute), undefined, viewHue.query)}
    {@render item(FilePlus, 'New', router.path === newRoute, () => layout.open(newRoute), undefined, viewHue.new)}

    {#each groups as g (g.group)}
      {#if collapsed}
        <div class="sep" role="separator"></div>
      {:else}
        <div class="group">{g.group}</div>
      {/if}
      {#each g.tools as t (t.id)}
        {@render item(t.icon, t.title, router.toolId === t.id, () => layout.openTool(t.id), undefined, t.hue, t.id)}
      {/each}
    {/each}
   </ContextMenu>
  </div>

  <!-- Three icons in a row while the rail is open (a column when it is
       collapsed): the list above keeps the room. -->
  <div class="bottom">
    {@render item(Settings2, 'Settings', router.path === settingsRoute, () => layout.open(settingsRoute), 'Mod+,', undefined, undefined, true)}
    {@render item(Keyboard, 'Keyboard shortcuts', false, () => (layout.shortcutsOpen = true), '?', undefined, undefined, true)}
    {@render item(
      collapsed ? PanelLeftOpen : PanelLeftClose,
      collapsed ? 'Expand sidebar' : 'Collapse sidebar',
      false,
      () => layout.toggleRail(),
      'Mod+B',
      undefined,
      undefined,
      true,
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
    /* The last entries fade out where the list scrolls on, so a cut-off
       row reads as "more below", not as a broken one. */
    padding-bottom: var(--s-3);
    mask-image: linear-gradient(to bottom, #000 calc(100% - var(--s-4)), transparent);
  }
  .group {
    margin: var(--s-3) 0 var(--s-1);
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
    height: calc(var(--control-h) + 4px);
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
  /* A tool's or view's icon in its hue, as far as the theme shows it
     (decision 94); the selected one in the accent otherwise. */
  .item.hued :global(svg) {
    color: var(--tool-icon);
  }
  .item.selected :global(svg) {
    color: var(--accent);
  }
  .item.hued.selected :global(svg) {
    color: var(--tool-accent);
  }
  /* Where the theme shows tool colours, the selected entry glows softly
     in its hue. */
  .item.hued.selected {
    background: color-mix(in oklab, var(--tool-accent-soft) var(--tint-fill), var(--bg-3));
    box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--tool-accent) calc(var(--tint-fill) * 0.3), transparent);
  }
  .bottom {
    display: flex;
    gap: 2px;
    padding-top: var(--s-2);
    border-top: 1px solid var(--border);
  }
  .collapsed .bottom {
    flex-direction: column;
  }
  .item.bare {
    justify-content: center;
    padding: 0;
  }
</style>
