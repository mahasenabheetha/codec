<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity'
  import ChevronsDownUp from '@lucide/svelte/icons/chevrons-down-up'
  import FileText from '@lucide/svelte/icons/file-text'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import SquareDashedMousePointer from '@lucide/svelte/icons/square-dashed-mouse-pointer'
  import Folder from '@lucide/svelte/icons/folder'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import FolderSearch from '@lucide/svelte/icons/folder-search'
  import FlaskConical from '@lucide/svelte/icons/flask-conical'
  import ListFilter from '@lucide/svelte/icons/list-filter'
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import Search from '@lucide/svelte/icons/search'
  import Button from '../../lib/components/Button.svelte'
  import ContextMenu, { type MenuEntry } from '../../lib/components/ContextMenu.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import TreeView, { type Row } from '../../lib/components/TreeView.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { router } from '../../lib/stores/router.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { comparison } from '../compare/compare.svelte'
  import FileIcon from './FileIcon.svelte'
  import { kindOf, lookOf } from './filetypes'
  import { buildTree, flatten, type TreeNode } from './tree'
  import { openFolder, openSample, workspace as ws } from './workspace.svelte'

  // Kinds hidden by the type filter; empty = show everything.
  const shown = new SvelteSet<string>()

  const kinds = $derived.by(() => {
    const counts = new Map<string, number>()
    for (const f of ws.files) counts.set(kindOf(f), (counts.get(kindOf(f)) ?? 0) + 1)
    return [...counts]
      .map(([kind, count]) => ({ kind, count, title: lookOf(kind).title }))
      .sort((a, b) => b.count - a.count || a.title.localeCompare(b.title))
  })

  const filtering = $derived(shown.size > 0)
  const visible = $derived(filtering ? ws.files.filter((f) => shown.has(kindOf(f))) : ws.files)
  const tree = $derived(buildTree(visible))
  const rows = $derived<Row<TreeNode>[]>(
    flatten(tree, ws.expanded, filtering).map(({ node, depth }) => ({
      id: node.path,
      depth,
      expandable: !node.file,
      expanded: !node.file && (filtering || ws.expanded.has(node.path)),
      data: node,
    })),
  )

  function toggle(path: string) {
    if (ws.expanded.has(path)) ws.expanded.delete(path)
    else ws.expanded.add(path)
  }

  // Files Ctrl/⌘+clicked, for "Compare selected". A new folder starts empty.
  const selection = new SvelteSet<string>()
  $effect(() => {
    void ws.info?.root
    selection.clear()
  })

  const base = (path: string) => path.split('/').pop() ?? path

  /** The context menu for the file row under the pointer. */
  function menuFor(e: MouseEvent): MenuEntry[] {
    const path = (e.target as HTMLElement).closest<HTMLElement>('[data-id]')?.dataset.id
    if (!path || !ws.byPath.has(path)) return []
    const out: MenuEntry[] = [{ label: 'Open', icon: FileText, run: () => layout.openFile(path) }, 'separator']
    const [a, b] = [...selection]
    if (selection.size === 2 && selection.has(path) && ws.byPath.has(a) && ws.byPath.has(b)) {
      out.push({ label: `Compare selected (${base(a)} ↔ ${base(b)})`, icon: GitCompare, run: () => comparison.openFiles(a, b) })
    }
    const picked = comparison.picked
    if (picked && picked !== path && ws.byPath.has(picked)) {
      out.push({ label: `Compare with ${base(picked)}`, icon: GitCompare, run: () => comparison.openFiles(picked, path) })
    }
    out.push({
      label: 'Select for compare',
      icon: SquareDashedMousePointer,
      run: () => {
        comparison.picked = path
        toast(`${base(path)} selected — right-click another file to compare`)
      },
    })
    return out
  }

  function toggleKind(kind: string) {
    if (shown.has(kind)) shown.delete(kind)
    else shown.add(kind)
  }
</script>

<aside class="explorer" aria-label="Explorer">
  <header>
    {#if ws.info?.open}
      <span class="name" title={ws.info.root}>{ws.info.name}</span>
    {:else}
      <span class="name muted">Explorer</span>
    {/if}
    <div class="actions">
      {#if ws.info?.open}
        <IconButton icon={Search} label="Go to file" shortcut="Mod+P" size="sm" onclick={() => (ws.quickOpen = true)} />
        <Popover side="bottom" align="end">
          {#snippet trigger(props)}
            <button {...props} type="button" class="filter-btn" class:active={filtering} aria-label="Filter by type">
              <ListFilter size={14} strokeWidth={1.75} />
            </button>
          {/snippet}
          <div class="filter">
            <div class="filter-head">
              <span>Show file types</span>
              {#if filtering}<button type="button" class="link" onclick={() => shown.clear()}>Show all</button>{/if}
            </div>
            {#each kinds as k (k.kind)}
              <label class="kind">
                <input type="checkbox" checked={shown.has(k.kind)} onchange={() => toggleKind(k.kind)} />
                <FileIcon file={k.kind} />
                <span class="kind-title">{k.title}</span>
                <span class="count">{k.count}</span>
              </label>
            {/each}
          </div>
        </Popover>
        <IconButton icon={ChevronsDownUp} label="Collapse folders" size="sm" onclick={() => ws.expanded.clear()} />
        <IconButton icon={RotateCw} label="Refresh" size="sm" onclick={() => ws.loadTree()} />
      {/if}
      <IconButton icon={FolderOpen} label="Open folder…" shortcut="Mod+O" size="sm" onclick={() => (ws.dialogOpen = true)} />
    </div>
  </header>

  <div class="body">
    {#if !ws.info?.open}
      <div class="empty-wrap">
        <EmptyState icon={FolderSearch} title="No folder open" description="Open a repository to browse its YAML. codec only reads it.">
          <Button variant="primary" size="sm" icon={FolderOpen} onclick={() => (ws.dialogOpen = true)}>Open folder</Button>
          <Button size="sm" icon={FlaskConical} onclick={openSample}>Try the sample</Button>
        </EmptyState>
      </div>
      {#if ws.info?.recent.length}
        <div class="recent">
          <div class="section">Recent</div>
          {#each ws.info.recent.slice(0, 6) as dir (dir)}
            <button type="button" class="recent-item" title={dir} onclick={() => openFolder(dir)}>
              <Folder size={14} strokeWidth={1.75} />
              <span>{dir.split(/[\\/]/).filter(Boolean).pop()}</span>
            </button>
          {/each}
        </div>
      {/if}
    {:else if ws.treeLoading && ws.files.length === 0}
      <div class="skeleton" aria-busy="true" aria-label="Loading files">
        {#each [70, 55, 80, 45, 65, 50, 75] as w, i (i)}<span style="width: {w}%"></span>{/each}
      </div>
    {:else if ws.treeError && ws.files.length === 0}
      <p class="error">{ws.treeError}</p>
    {:else if rows.length === 0}
      <p class="hint">{filtering ? 'No files of the selected types.' : 'No files here (after .gitignore).'}</p>
    {:else}
      <ContextMenu label="File actions" items={menuFor}>
        <TreeView
          {rows}
          label="Files in {ws.info.name}"
          active={router.filePath}
          ontoggle={toggle}
          onopen={(node) => layout.openFile(node.path)}
          {selection}
          selectable={(node) => !!node.file}
          dragText={(node) => (node.file ? node.path : null)}
        >
          {#snippet row(node)}
            {#if node.file}
              <FileIcon file={node.file} />
              <span class="label" title={node.path + ' · ' + lookOf(node.file).title}>{node.name}</span>
            {:else}
              <span class="folder"><Folder size={14} strokeWidth={1.75} /></span>
              <span class="label">{node.name}</span>
            {/if}
          {/snippet}
        </TreeView>
      </ContextMenu>
      {#if ws.truncated}
        <p class="hint">Showing the first 50,000 files.</p>
      {/if}
    {/if}
  </div>
</aside>

<style>
  .explorer {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-1);
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--tabbar-h);
    padding: 0 var(--s-1) 0 var(--s-3);
    border-bottom: 1px solid var(--border);
    flex: 0 0 auto;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .muted {
    color: var(--fg-2);
  }
  .actions {
    display: flex;
    align-items: center;
  }
  .filter-btn {
    display: inline-grid;
    place-items: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    padding: 0;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
  }
  .filter-btn:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .filter-btn.active {
    color: var(--accent);
    background: var(--accent-soft);
  }
  .filter {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 240px;
    max-height: 60vh;
    overflow-y: auto;
  }
  .filter-head {
    display: flex;
    justify-content: space-between;
    padding: 0 var(--s-1) var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .link {
    padding: 0;
    font-size: var(--fs-xs);
    color: var(--accent);
    background: none;
    border: none;
    text-transform: none;
    letter-spacing: 0;
  }
  .kind {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 28px;
    padding: 0 var(--s-1);
    border-radius: var(--r-sm);
    cursor: pointer;
  }
  .kind:hover {
    background: var(--bg-3);
  }
  .kind input {
    accent-color: var(--accent);
    margin: 0;
  }
  .kind-title {
    flex: 1;
  }
  .count {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    font-variant-numeric: tabular-nums;
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  /* EmptyState fills its parent; keep it compact so recents show below. */
  .empty-wrap {
    height: 240px;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .folder {
    display: grid;
    color: var(--fg-2);
  }
  .skeleton {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
  }
  .skeleton span {
    height: 12px;
    border-radius: var(--r-sm);
    background: var(--bg-3);
    animation: pulse 1.2s var(--ease) infinite alternate;
  }
  @keyframes pulse {
    to {
      opacity: 0.4;
    }
  }
  .hint,
  .error {
    padding: var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .error {
    color: var(--err);
  }
  .recent {
    padding: 0 var(--s-2) var(--s-4);
  }
  .section {
    padding: var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .recent-item {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    height: 28px;
    padding: 0 var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
    text-align: left;
  }
  .recent-item:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .recent-item span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
