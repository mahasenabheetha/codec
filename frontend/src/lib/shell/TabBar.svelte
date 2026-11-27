<script lang="ts">
  import Search from '@lucide/svelte/icons/search'
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal'
  import X from '@lucide/svelte/icons/x'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ArrowRightToLine from '@lucide/svelte/icons/arrow-right-to-line'
  import Layers2 from '@lucide/svelte/icons/layers-2'
  import type { Component } from 'svelte'
  import ContextMenu, { type MenuEntry } from '../components/ContextMenu.svelte'
  import Popover from '../components/Popover.svelte'
  import Kbd from '../components/Kbd.svelte'
  import FileIcon from '../../features/workspace/FileIcon.svelte'
  import { workspace } from '../../features/workspace/workspace.svelte'
  import { layout } from '../stores/layout.svelte'
  import {
    compareRoute,
    settingsRoute,
    problemsRoute,
    newRoute,
    queryRoute,
    router,
    routeFile,
    routeHelm,
    routeK8s,
    routeKustomize,
    routeArgo,
    routeCI,
    routeCompose,
    routeAnsible,
    routeClone,
    routeTool,
  } from '../stores/router.svelte'
  import { charts } from '../../features/helm/helm.svelte'
  import { hueVar, toolById, viewHue, viewIcon, type Hue } from '../tools'

  // Open tools and files as tabs. Middle-click closes, like an editor.
  function onauxclick(e: MouseEvent, route: string) {
    if (e.button === 1) {
      e.preventDefault()
      layout.close(route)
    }
  }

  // File tabs show the file name; when two open files share a name,
  // their parent folder is added to tell them apart.
  const names = $derived.by(() => {
    const files = layout.tabs.map(routeFile).filter((p): p is string => p !== null)
    const count = new Map<string, number>()
    for (const p of files) count.set(base(p), (count.get(base(p)) ?? 0) + 1)
    return new Map(
      files.map((p) => {
        const parts = p.split('/')
        const hint = count.get(base(p))! > 1 && parts.length > 1 ? parts[parts.length - 2] : ''
        return [p, { name: base(p), hint }]
      }),
    )
  })

  /** What a tab shows: its name, its icon (a file's or a chart's logo
   *  when it has none) and, as on the rail, its colour. */
  interface Tab {
    title: string
    icon?: Component
    hue?: Hue
    file?: string
    chart?: string
    hint?: string
  }

  // Fixed views.
  const views: Record<string, Tab> = {
    [problemsRoute]: { title: 'Problems', icon: viewIcon.problems, hue: viewHue.problems },
    [settingsRoute]: { title: 'Settings', icon: SlidersHorizontal },
    [compareRoute]: { title: 'Compare', icon: viewIcon.compare, hue: viewHue.compare },
    [queryRoute]: { title: 'Query', icon: viewIcon.query, hue: viewHue.query },
    [newRoute]: { title: 'New', icon: viewIcon.new, hue: viewHue.new },
  }

  // Views over a folder or a file: Kubernetes resources, Kustomize
  // builds and the lens views.
  function scopedView(route: string): Tab | undefined {
    const k8s = routeK8s(route)
    if (k8s !== null) return { title: `K8s · ${k8s === '.' ? 'all' : base(k8s)}`, icon: viewIcon.k8s, hue: viewHue.k8s }
    const kz = routeKustomize(route)
    if (kz !== null) return { title: `Kustomize · ${base(kz)}`, icon: viewIcon.kustomize, hue: viewHue.kustomize }
    const ar = routeArgo(route)
    if (ar !== null) return { title: `Argo · ${base(ar)}`, icon: viewIcon.argo, hue: viewHue.argo }
    const cr = routeCI(route)
    if (cr !== null) return { title: `Pipeline · ${base(cr)}`, icon: viewIcon.ci, hue: viewHue.ci }
    const co = routeCompose(route)
    if (co !== null) return { title: `Compose · ${base(co)}`, icon: viewIcon.compose, hue: viewHue.compose }
    const an = routeAnsible(route)
    if (an !== null) return { title: `Ansible · ${base(an)}`, icon: viewIcon.ansible, hue: viewHue.ansible }
    const cl = routeClone(route)
    if (cl !== null) return { title: `Clone · ${base(cl)}`, icon: viewIcon.clone, hue: viewHue.clone }
    return undefined
  }

  function base(p: string) {
    return p.slice(p.lastIndexOf('/') + 1)
  }

  /** A route's tab; undefined for a route with no tab. */
  function tabOf(route: string): Tab | undefined {
    const tool = toolById(routeTool(route))
    if (tool) return { title: tool.title, icon: tool.icon, hue: tool.hue }
    const view = views[route] ?? scopedView(route)
    if (view) return view
    const file = routeFile(route)
    if (file) return { title: names.get(file)?.name ?? base(file), hue: viewHue.explorer, file, hint: names.get(file)?.hint }
    const chart = routeHelm(route)
    if (chart) return { title: 'Helm · ' + (charts.byPath(chart)?.name ?? chart), hue: viewHue.helm, chart }
    return undefined
  }

  // Right-click (or Shift+F10) on a tab.
  function menuFor(e: MouseEvent): MenuEntry[] {
    const route = (e.target as HTMLElement).closest<HTMLElement>('[data-route]')?.dataset.route
    if (!route) return []
    const i = layout.tabs.indexOf(route)
    return [
      { label: 'Close', icon: X, run: () => layout.close(route) },
      { label: 'Close the others', icon: Layers2, disabled: layout.tabs.length < 2, run: () => layout.closeOthers(route) },
      { label: 'Close tabs to the right', icon: ArrowRightToLine, disabled: i === layout.tabs.length - 1, run: () => layout.closeRight(route) },
      'separator',
      { label: 'Close all', icon: X, run: () => layout.closeAll() },
    ]
  }

  // Drag a tab along the bar to move it. A line marks where it would
  // land: after the tab it is over when moving right, before it when
  // moving left (as moveTab places it).
  let dragged = $state<string | null>(null)
  let over = $state<string | null>(null)
  const landing = $derived(
    dragged && over && over !== dragged ? (layout.tabs.indexOf(dragged) < layout.tabs.indexOf(over) ? 'after' : 'before') : null,
  )
  function drop(route: string) {
    if (dragged) layout.moveTab(dragged, route)
    dragged = over = null
  }

  // More tabs than fit: the bar scrolls (the wheel moves it sideways),
  // the current tab is kept in view, and "+N" at the end lists the ones
  // out of sight.
  let strip = $state<HTMLDivElement>()
  let hidden = $state<string[]>([])
  let listOpen = $state(false)
  function measure() {
    const el = strip
    if (!el) return
    if (el.scrollWidth <= el.clientWidth + 1) {
      hidden = []
      return
    }
    const box = el.getBoundingClientRect()
    hidden = [...el.querySelectorAll<HTMLElement>('.tab')]
      .filter((t) => {
        const r = t.getBoundingClientRect()
        return r.left < box.left - 1 || r.right > box.right + 1
      })
      .map((t) => t.dataset.route!)
  }
  $effect(() => {
    if (!strip) return
    const ro = new ResizeObserver(measure)
    ro.observe(strip)
    void layout.tabs.length
    queueMicrotask(measure)
    return () => ro.disconnect()
  })
  $effect(() => {
    void router.path
    void layout.tabs.length
    queueMicrotask(() => strip?.querySelector('.tab.selected')?.scrollIntoView({ block: 'nearest', inline: 'nearest' }))
  })
  function onwheel(e: WheelEvent) {
    if (!strip || !hidden.length || e.deltaX || e.shiftKey) return
    strip.scrollLeft += e.deltaY
    e.preventDefault()
  }
</script>

{#snippet glyph(t: Tab)}
  {#if t.icon}
    <t.icon size={14} />
  {:else if t.chart}
    <FileIcon file="helm-chart" />
  {:else}
    <FileIcon file={workspace.byPath.get(t.file!) ?? { lang: 'text' }} />
  {/if}
{/snippet}

<div class="tabbar">
  <div class="tabs" role="tablist" aria-label="Open tabs" bind:this={strip} {onwheel} onscroll={measure}>
   <ContextMenu label="Tab" items={menuFor}>
    {#each layout.tabs as route (route)}
      {@const t = tabOf(route)}
      {#if t}
        {@const selected = router.path === route}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="tab {(over === route && landing) || ''}"
          class:selected
          class:hued={!!t.hue}
          class:dragging={dragged === route}
          style:--tool={t.hue && hueVar(t.hue)}
          title={t.file ?? (t.chart ? `Helm view of ${t.chart}` : undefined)}
          data-route={route}
          draggable="true"
          ondragstart={(e) => {
            dragged = route
            e.dataTransfer?.setData('text/plain', t.title)
            if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
          }}
          ondragover={(e) => {
            if (!dragged) return
            e.preventDefault()
            over = route
          }}
          ondragleave={() => over === route && (over = null)}
          ondrop={(e) => {
            e.preventDefault()
            drop(route)
          }}
          ondragend={() => (dragged = over = null)}
        >
          <button
            type="button"
            role="tab"
            aria-selected={selected}
            class="tab-main"
            onclick={() => router.go(route)}
            onauxclick={(e) => onauxclick(e, route)}
          >
            {@render glyph(t)}
            {t.title}
            {#if t.hint}<span class="hint">{t.hint}</span>{/if}
            {#if t.file && workspace.dirty.has(t.file)}<span class="dirty" title="What-if edits (not saved)"></span>{/if}
          </button>
          <button type="button" class="close" aria-label="Close {t.title}" onclick={() => layout.close(route)}>
            <X size={12} />
          </button>
        </div>
      {/if}
    {/each}
   </ContextMenu>
  </div>

  {#if hidden.length}
    <Popover side="bottom" align="end" bind:open={listOpen}>
      {#snippet trigger(props)}
        <button {...props} type="button" class="more" aria-label="{hidden.length} more tabs" title="Tabs out of sight">
          +{hidden.length}
          <ChevronDown size={13} />
        </button>
      {/snippet}
      <ul class="tablist" aria-label="Tabs out of sight">
        {#each hidden as route (route)}
          {@const t = tabOf(route)}
          {#if t}
            <li class:hued={!!t.hue} class:current={router.path === route} style:--tool={t.hue && hueVar(t.hue)}>
              <button
                type="button"
                class="pick"
                onclick={() => {
                  listOpen = false
                  router.go(route)
                }}
              >
                {@render glyph(t)}
                <span class="name">{t.title}</span>
                {#if t.hint}<span class="hint">{t.hint}</span>{/if}
              </button>
              <button type="button" class="close" aria-label="Close {t.title}" onclick={() => layout.close(route)}><X size={12} /></button>
            </li>
          {/if}
        {/each}
      </ul>
    </Popover>
  {/if}

  <button type="button" class="palette" onclick={() => (layout.paletteOpen = true)}>
    <Search size={14} />
    <span class="label">Search tools and actions</span>
    <Kbd combo="Mod+K" />
  </button>
</div>

<style>
  .tabbar {
    display: flex;
    align-items: stretch;
    gap: var(--s-2);
    height: var(--tabbar-h);
    padding-right: var(--s-2);
    background: var(--bg-0);
    border-bottom: 1px solid var(--border);
  }
  .tabs {
    flex: 1;
    display: flex;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tab {
    position: relative;
    display: flex;
    align-items: center;
    flex: 0 0 auto;
    border-right: 1px solid var(--border);
    color: var(--fg-1);
    transition: background var(--dur) var(--ease);
  }
  .tab:hover:not(.selected) {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .tab.selected {
    color: var(--fg-0);
    background: var(--bg-1);
  }
  .tab.selected::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    height: 2px;
    background: var(--tool-accent, var(--accent));
  }
  .tab.dragging {
    opacity: 0.35;
  }
  /* Where a dragged tab would land. */
  .tab.before::after,
  .tab.after::after {
    content: '';
    position: absolute;
    top: var(--s-2);
    bottom: var(--s-2);
    z-index: 1;
    width: 2px;
    background: var(--accent);
    border-radius: var(--r-full);
  }
  .tab.before::after {
    left: -1px;
  }
  .tab.after::after {
    right: -1px;
  }
  /* A tab's icon in its tool's or view's colour (the accent for
     Settings), as far as the theme shows it (decision 94). */
  .tab-main > :global(svg.lucide-icon) {
    color: var(--tool-icon, var(--accent));
  }
  .tab-main {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 100%;
    padding: 0 var(--s-1) 0 var(--s-3);
    font-size: var(--fs-md);
    color: inherit;
    background: none;
    border: none;
  }
  .tab.selected .tab-main {
    font-weight: var(--fw-medium);
  }
  .hint {
    font-size: var(--fs-sm);
    font-weight: var(--fw-regular);
    color: var(--fg-2);
  }
  .dirty {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--warn);
  }
  .close {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    margin-right: var(--s-2);
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
  }
  .tab:hover .close,
  .tab.selected .close,
  .close:focus-visible {
    opacity: 1;
  }
  .tab:hover:not(.selected) .close {
    color: var(--fg-1);
  }
  .close:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  /* The tabs out of sight, when they don't all fit. */
  .more {
    align-self: center;
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 2px;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
  }
  .more:hover,
  .more[aria-expanded='true'] {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .tablist {
    list-style: none;
    margin: calc(var(--s-2) * -1);
    padding: 0;
    max-width: 360px;
    max-height: 60vh;
    overflow-y: auto;
  }
  .tablist li {
    display: flex;
    align-items: center;
    border-radius: var(--r-md);
  }
  .tablist li:hover {
    background: var(--bg-3);
  }
  .tablist li.current {
    background: var(--tool-accent-soft, var(--accent-soft));
  }
  .pick {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: calc(var(--control-h) - 2px);
    padding: 0 var(--s-2);
    overflow: hidden;
    white-space: nowrap;
    text-align: left;
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .pick > :global(svg.lucide-icon) {
    flex: none;
    color: var(--tool-icon, var(--accent));
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tablist .close {
    margin-right: var(--s-1);
  }
  .tablist li:hover .close,
  .tablist li:focus-within .close {
    opacity: 1;
  }
  .palette {
    align-self: center;
    flex: 0 1 260px;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h-sm);
    padding: 0 var(--s-1) 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .palette:hover {
    color: var(--fg-1);
    border-color: var(--fg-2);
  }
  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }
  @media (max-width: 720px) {
    .palette {
      flex: none;
    }
    .palette .label {
      display: none;
    }
  }
</style>
