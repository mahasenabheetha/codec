<script lang="ts">
  import Search from '@lucide/svelte/icons/search'
  import Boxes from '@lucide/svelte/icons/boxes'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import Layers from '@lucide/svelte/icons/layers'
  import Workflow from '@lucide/svelte/icons/workflow'
  import GitBranch from '@lucide/svelte/icons/git-branch'
  import Container from '@lucide/svelte/icons/container'
  import ListTree from '@lucide/svelte/icons/list-tree'
  import ListChecks from '@lucide/svelte/icons/list-checks'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import FilePlus from '@lucide/svelte/icons/file-plus'
  import CopyPlus from '@lucide/svelte/icons/copy-plus'
  import Settings2 from '@lucide/svelte/icons/settings-2'
  import X from '@lucide/svelte/icons/x'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ArrowRightToLine from '@lucide/svelte/icons/arrow-right-to-line'
  import Layers2 from '@lucide/svelte/icons/layers-2'
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
  import { hueVar, toolById, viewHue, type Hue } from '../tools'

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

  // Fixed views: title, icon and, as on the rail, their hue.
  const views: Record<string, { title: string; icon: typeof X; hue?: Hue }> = {
    [problemsRoute]: { title: 'Problems', icon: ListChecks, hue: viewHue.problems },
    [settingsRoute]: { title: 'Settings', icon: Settings2 },
    [compareRoute]: { title: 'Compare', icon: GitCompare, hue: viewHue.compare },
    [queryRoute]: { title: 'Query', icon: TextSearch, hue: viewHue.query },
    [newRoute]: { title: 'New', icon: FilePlus, hue: viewHue.new },
  }

  // Views over a folder: Kubernetes resources and Kustomize builds.
  function scopedView(route: string): { title: string; icon: typeof X; hue?: Hue } | undefined {
    const k8s = routeK8s(route)
    if (k8s !== null) return { title: `K8s · ${k8s === '.' ? 'all' : base(k8s)}`, icon: Boxes }
    const kz = routeKustomize(route)
    if (kz !== null) return { title: `Kustomize · ${base(kz)}`, icon: Layers }
    const ar = routeArgo(route)
    if (ar !== null) return { title: `Argo · ${base(ar)}`, icon: Workflow }
    const cr = routeCI(route)
    if (cr !== null) return { title: `Pipeline · ${base(cr)}`, icon: GitBranch }
    const co = routeCompose(route)
    if (co !== null) return { title: `Compose · ${base(co)}`, icon: Container }
    const an = routeAnsible(route)
    if (an !== null) return { title: `Ansible · ${base(an)}`, icon: ListTree }
    const cl = routeClone(route)
    if (cl !== null) return { title: `Clone · ${base(cl)}`, icon: CopyPlus }
    return undefined
  }

  function base(p: string) {
    return p.slice(p.lastIndexOf('/') + 1)
  }

  /** A tab's name, as the bar shows it; undefined for a route with no tab. */
  function titleOf(route: string): string | undefined {
    const tool = toolById(routeTool(route))
    if (tool) return tool.title
    const view = views[route] ?? scopedView(route)
    if (view) return view.title
    const file = routeFile(route)
    if (file) return names.get(file)?.name
    const chart = routeHelm(route)
    if (chart) return 'Helm · ' + (charts.byPath(chart)?.name ?? chart)
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

  // Drag a tab along the bar to move it.
  let dragged = $state<string | null>(null)
  let over = $state<string | null>(null)
  function drop(route: string) {
    if (dragged) layout.moveTab(dragged, route)
    dragged = over = null
  }

  // More tabs than fit: the bar scrolls (the wheel moves it sideways),
  // the current tab is kept in view, and a menu at the end lists them all.
  let strip = $state<HTMLDivElement>()
  let overflowing = $state(false)
  let listOpen = $state(false)
  $effect(() => {
    if (!strip) return
    const el = strip
    const measure = () => (overflowing = el.scrollWidth > el.clientWidth + 1)
    const ro = new ResizeObserver(measure)
    ro.observe(el)
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
    if (!strip || !overflowing || e.deltaX || e.shiftKey) return
    strip.scrollLeft += e.deltaY
    e.preventDefault()
  }
</script>

<div class="tabbar">
  <div class="tabs" role="tablist" aria-label="Open tabs" bind:this={strip} {onwheel}>
   <ContextMenu label="Tab" items={menuFor}>
    {#each layout.tabs as route (route)}
      {@const tool = toolById(routeTool(route))}
      {@const file = routeFile(route)}
      {@const chart = routeHelm(route)}
      {@const view = views[route] ?? scopedView(route)}
      {#if tool || file || chart || view}
        {@const selected = router.path === route}
        {@const title = tool ? tool.title : view ? view.title : file ? names.get(file)?.name : `Helm · ${charts.byPath(chart!)?.name ?? chart}`}
        {@const hue = tool ? tool.hue : view?.hue}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="tab"
          class:selected
          class:hued={!!hue}
          class:over={over === route && dragged !== route}
          style:--tool={hue && hueVar(hue)}
          title={file ?? (chart ? `Helm view of ${chart}` : undefined)}
          data-route={route}
          draggable="true"
          ondragstart={(e) => {
            dragged = route
            e.dataTransfer?.setData('text/plain', title ?? route)
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
            {#if tool}
              <tool.icon size={14} strokeWidth={1.75} />
            {:else if view}
              <view.icon size={14} strokeWidth={1.75} />
            {:else if chart}
              <FileIcon file="helm-chart" />
            {:else}
              <FileIcon file={workspace.byPath.get(file!) ?? { lang: 'text' }} />
            {/if}
            {title}
            {#if file && names.get(file)?.hint}<span class="hint">{names.get(file)?.hint}</span>{/if}
            {#if file && workspace.dirty.has(file)}<span class="dirty" title="What-if edits (not saved)"></span>{/if}
          </button>
          <button type="button" class="close" aria-label="Close {title}" onclick={() => layout.close(route)}>
            <X size={12} strokeWidth={2} />
          </button>
        </div>
      {/if}
    {/each}
   </ContextMenu>
  </div>

  {#if overflowing}
    <Popover side="bottom" align="end" bind:open={listOpen}>
      {#snippet trigger(props)}
        <button {...props} type="button" class="all" aria-label="All open tabs ({layout.tabs.length})" title="All open tabs">
          {layout.tabs.length}
          <ChevronDown size={13} strokeWidth={2} />
        </button>
      {/snippet}
      <ul class="tablist" aria-label="Open tabs">
        {#each layout.tabs as route (route)}
          {@const name = titleOf(route)}
          {@const hint = names.get(routeFile(route) ?? '')?.hint}
          {#if name}
            <li class:current={router.path === route}>
              <button
                type="button"
                class="pick"
                onclick={() => {
                  listOpen = false
                  router.go(route)
                }}>{name}{#if hint}<span class="hint">{hint}</span>{/if}</button
              >
              <button type="button" class="close shown" aria-label="Close {name}" onclick={() => layout.close(route)}><X size={12} strokeWidth={2} /></button>
            </li>
          {/if}
        {/each}
      </ul>
    </Popover>
  {/if}

  <button type="button" class="palette" onclick={() => (layout.paletteOpen = true)}>
    <Search size={14} strokeWidth={1.75} />
    <span>Search tools and actions</span>
    <Kbd combo="Mod+K" />
  </button>
</div>

<style>
  .tabbar {
    display: flex;
    align-items: stretch;
    gap: var(--s-3);
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
  }
  .tab.selected {
    color: var(--fg-0);
    background: var(--bg-1);
  }
  /* Where a dragged tab would land. */
  .tab.over {
    box-shadow: inset 2px 0 0 var(--tool-accent, var(--accent));
  }
  .tab.selected::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    height: 2px;
    background: var(--accent);
  }
  /* A tool's or view's tab in its hue, as on the rail (decision 94):
     the icon tinted, and the selected tab's bar and glow in the hue. */
  .tab.hued .tab-main > :global(svg) {
    color: var(--tool-icon);
  }
  .tab.hued.selected .tab-main > :global(svg) {
    color: var(--tool-accent);
  }
  .tab.hued.selected {
    background: color-mix(in oklab, var(--tool-accent-soft) var(--tint-fill), var(--bg-1));
  }
  .tab.hued.selected::before {
    background: var(--tool-accent);
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
  .tab-main:hover {
    color: var(--fg-0);
  }
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .dirty {
    width: 6px;
    height: 6px;
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
  .close:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  /* All open tabs, when they don't fit. */
  .all {
    align-self: center;
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 2px;
    height: 26px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .all:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  .tablist {
    list-style: none;
    margin: 0;
    padding: var(--s-1);
    min-width: 240px;
    max-width: 360px;
    max-height: 60vh;
    overflow-y: auto;
  }
  .tablist li {
    display: flex;
    align-items: center;
    border-radius: var(--r-md);
  }
  .tablist li:hover,
  .tablist li.current {
    background: var(--bg-3);
  }
  .pick {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    font-size: var(--fs-md);
    color: var(--fg-1);
    background: none;
    border: none;
  }
  li.current .pick {
    color: var(--fg-0);
    font-weight: var(--fw-medium);
  }
  .close.shown {
    opacity: 1;
    margin-right: var(--s-1);
  }
  .palette {
    align-self: center;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 26px;
    padding: 0 var(--s-1) 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .palette:hover {
    color: var(--fg-1);
    border-color: var(--border-strong);
  }
  @media (max-width: 720px) {
    .palette span {
      display: none;
    }
  }
</style>
