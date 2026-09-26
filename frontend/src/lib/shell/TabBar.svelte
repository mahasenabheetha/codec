<script lang="ts">
  import Search from '@lucide/svelte/icons/search'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import ListChecks from '@lucide/svelte/icons/list-checks'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import Settings2 from '@lucide/svelte/icons/settings-2'
  import X from '@lucide/svelte/icons/x'
  import Kbd from '../components/Kbd.svelte'
  import FileIcon from '../../features/workspace/FileIcon.svelte'
  import { workspace } from '../../features/workspace/workspace.svelte'
  import { layout } from '../stores/layout.svelte'
  import { compareRoute, lintSettingsRoute, problemsRoute, queryRoute, router, routeFile, routeHelm, routeTool } from '../stores/router.svelte'
  import { charts } from '../../features/helm/helm.svelte'
  import { toolById } from '../tools'

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

  // Fixed views: title and icon.
  const views: Record<string, { title: string; icon: typeof X }> = {
    [problemsRoute]: { title: 'Problems', icon: ListChecks },
    [lintSettingsRoute]: { title: 'Lint settings', icon: Settings2 },
    [compareRoute]: { title: 'Compare', icon: GitCompare },
    [queryRoute]: { title: 'Query', icon: TextSearch },
  }

  function base(p: string) {
    return p.slice(p.lastIndexOf('/') + 1)
  }
</script>

<div class="tabbar">
  <div class="tabs" role="tablist" aria-label="Open tabs">
    {#each layout.tabs as route (route)}
      {@const tool = toolById(routeTool(route))}
      {@const file = routeFile(route)}
      {@const chart = routeHelm(route)}
      {@const view = views[route]}
      {#if tool || file || chart || view}
        {@const selected = router.path === route}
        {@const title = tool ? tool.title : view ? view.title : file ? names.get(file)?.name : `Helm · ${charts.byPath(chart!)?.name ?? chart}`}
        <div class="tab" class:selected title={file ?? (chart ? `Helm view of ${chart}` : undefined)}>
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
  </div>

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
  .tab.selected::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    height: 2px;
    background: var(--accent);
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
