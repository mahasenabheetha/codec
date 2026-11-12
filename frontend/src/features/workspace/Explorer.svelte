<script lang="ts">
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
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
  import Logs from '@lucide/svelte/icons/logs'
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import Search from '@lucide/svelte/icons/search'
  import Boxes from '@lucide/svelte/icons/boxes'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Container from '@lucide/svelte/icons/container'
  import Copy from '@lucide/svelte/icons/copy'
  import CopyPlus from '@lucide/svelte/icons/copy-plus'
  import GitBranch from '@lucide/svelte/icons/git-branch'
  import Layers from '@lucide/svelte/icons/layers'
  import Link from '@lucide/svelte/icons/link'
  import ListTree from '@lucide/svelte/icons/list-tree'
  import PencilLine from '@lucide/svelte/icons/pencil-line'
  import Ship from '@lucide/svelte/icons/ship'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import Workflow from '@lucide/svelte/icons/workflow'
  import Button from '../../lib/components/Button.svelte'
  import ContextMenu, { type MenuEntry } from '../../lib/components/ContextMenu.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import TreeView, { type Row } from '../../lib/components/TreeView.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { ansibleRoute, argoRoute, ciRoute, cloneRoute, composeRoute, k8sRoute, kustomizeRoute, router } from '../../lib/stores/router.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { queries } from '../compare/compare.svelte'
  import { charts } from '../helm/helm.svelte'
  import { lint } from '../lint/lint.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { comparison } from '../compare/compare.svelte'
  import { logTabs } from '../logs/viewer.svelte'
  import FileIcon from './FileIcon.svelte'
  import { kindOf, lookOf } from './filetypes'
  import { folderIconUrl } from './icons.svelte'
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

  // What is known about each file, shown at the end of its row and
  // summed up its folders (3.10.2): what-if edits in open tabs, and the
  // errors and warnings of the last workspace check (Problems), if one
  // was run.
  type Marks = { e: number; w: number; whatif: number }
  const marks = $derived.by(() => {
    const out = new Map<string, Marks>()
    const add = (path: string, m: Partial<Marks>) => {
      // The file, then each folder above it.
      for (let p = path; p; p = p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : '') {
        const at = out.get(p) ?? { e: 0, w: 0, whatif: 0 }
        at.e += m.e ?? 0
        at.w += m.w ?? 0
        at.whatif += m.whatif ?? 0
        out.set(p, at)
      }
    }
    for (const path of ws.dirty) add(path, { whatif: 1 })
    for (const file of lint.results ?? []) {
      let e = 0
      let w = 0
      for (const d of file.diagnostics) {
        if (d.severity === 'error') e++
        else if (d.severity === 'warning') w++
      }
      if (e || w) add(file.path, { e, w })
    }
    return out
  })
  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`

  function copyPath(path: string, absolute: boolean) {
    if (!ws.info?.root) return
    const sep = ws.info.sep
    copyText(absolute ? ws.info.root + sep + path.split('/').join(sep) : path, absolute ? 'Full path' : 'Path')
  }
  const pathEntries = (path: string): MenuEntry[] => [
    { label: 'Copy path', icon: Link, run: () => copyPath(path, false) },
    { label: 'Copy full path', icon: Copy, run: () => copyPath(path, true) },
  ]

  /** A folder's menu: the views over it, and its path. */
  function folderMenu(path: string): MenuEntry[] {
    const out: MenuEntry[] = []
    const chart = charts.byPath(path)
    if (chart) out.push({ label: `Render the chart ${chart.name}`, icon: Ship, run: () => layout.openHelm(chart.path) })
    if (['kustomization.yaml', 'kustomization.yml', 'Kustomization'].some((n) => ws.byPath.has(path + '/' + n)))
      out.push({ label: 'Build this kustomization', icon: Layers, run: () => layout.open(kustomizeRoute(path)) })
    out.push({ label: 'Kubernetes resources in this folder', icon: Boxes, run: () => layout.open(k8sRoute(path)) })
    out.push('separator', ...pathEntries(path))
    return out
  }

  /** The view of a file's own type, first in its menu. */
  function viewEntries(path: string): MenuEntry[] {
    const type = ws.byPath.get(path)?.type ?? ''
    const out: MenuEntry[] = []
    const chart = charts.of(path)
    if (chart) out.push({ label: `Render the chart ${chart.name}`, icon: Ship, run: () => layout.openHelm(chart.path) })
    else if (type === 'kustomize') out.push({ label: 'Build this kustomization', icon: Layers, run: () => layout.open(kustomizeRoute(path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '.')) })
    if (type === 'github-actions' || type === 'gitlab-ci' || type === 'azure-pipelines') out.push({ label: 'Open as a pipeline', icon: GitBranch, run: () => layout.open(ciRoute(path)) })
    if (type === 'compose') out.push({ label: 'Open the Compose view', icon: Container, run: () => layout.open(composeRoute(path)) })
    if (type === 'ansible-playbook' || type === 'ansible-inventory') out.push({ label: 'Open the Ansible view', icon: ListTree, run: () => layout.open(ansibleRoute(path)) })
    if (type === 'argo-workflows' || type === 'argocd') out.push({ label: 'Open the Argo view', icon: Workflow, run: () => layout.open(argoRoute(path)) })
    return out
  }
  // Files the Log viewer can open from here (bigger ones aren't listed; open them as files there).
  const isLog = (path: string) => /\.(log|ndjson|jsonl|clef|out|txt)(\.gz)?$|\.log\.\d+$/i.test(path)

  /** The context menu for the file row under the pointer. */
  function menuFor(e: MouseEvent): MenuEntry[] {
    const path = (e.target as HTMLElement).closest<HTMLElement>('[data-id]')?.dataset.id
    if (!path) return []
    if (!ws.byPath.has(path)) return folderMenu(path)
    const out: MenuEntry[] = [{ label: 'Open', icon: FileText, run: () => layout.openFile(path) }, ...viewEntries(path)]
    if (isLog(path)) {
      out.push({
        label: 'Open in Log viewer',
        icon: Logs,
        run: () => {
          logTabs.open([{ kind: 'path', path }])
          layout.openTool('logs')
        },
      })
    }
    out.push('separator')
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
    if (/\.(ya?ml|json)$/i.test(path)) {
      out.push('separator')
      out.push({ label: 'Query this file (jq)', icon: TextSearch, run: () => queries.open({ path }) })
      out.push({ label: 'Clone with a new name…', icon: CopyPlus, run: () => layout.open(cloneRoute(path)) })
    }
    out.push('separator', ...pathEntries(path))
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
      <IconButton icon={FolderOpen} tone="open" label="Open folder…" shortcut="Mod+O" size="sm" onclick={() => ws.chooseFolder()} />
    </div>
  </header>

  <div class="body">
    {#if !ws.info?.open}
      <div class="empty-wrap">
        <EmptyState icon={FolderSearch} title="No folder open" description="Open a repository to browse its YAML. codec only reads it.">
          <Button variant="primary" size="sm" icon={FolderOpen} tone="open" onclick={() => ws.chooseFolder()}>Open folder</Button>
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
      <ErrorNote message={ws.treeError} retry={() => ws.loadTree()} />
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
          {#snippet row(node, r)}
            {@const folderSrc = node.file ? null : folderIconUrl(node.name, r.expanded)}
            {#if node.file}
              <FileIcon file={node.file} />
              <span class="label" title={node.path + ' · ' + lookOf(node.file).title}>{node.name}</span>
            {:else}
              <span class="folder">
                {#if folderSrc}<img src={folderSrc} alt="" draggable="false" />{:else}<Folder size={14} strokeWidth={1.75} />{/if}
              </span>
              <span class="label">{node.name}</span>
            {/if}
            {@const m = marks.get(node.path)}
            {#if m}
              <span class="marks">
                {#if m.whatif}
                  <span class="wi" title={node.file ? 'What-if edits (not saved)' : `${plural(m.whatif, 'file')} with what-if edits`}><PencilLine size={11} strokeWidth={2} /><span class="sr-only">what-if edits</span></span>
                {/if}
                {#if m.e}<span class="e" title={plural(m.e, 'error')}><CircleX size={11} strokeWidth={2} />{m.e}<span class="sr-only"> errors</span></span>{/if}
                {#if m.w}<span class="w" title={plural(m.w, 'warning')}><TriangleAlert size={11} strokeWidth={2} />{m.w}<span class="sr-only"> warnings</span></span>{/if}
              </span>
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
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* What is known about a file, at the end of its row: an icon with
     each count, so it is never colour alone. */
  .marks {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    padding: 0 var(--s-2) 0 var(--s-1);
    font-size: var(--fs-xs);
  }
  .marks > span {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }
  .wi,
  .w {
    color: var(--warn);
  }
  .e {
    color: var(--err);
  }
  .folder {
    display: grid;
    color: var(--fg-2);
  }
  .folder img {
    width: round(calc(14px * var(--icon-scale)), 1px);
    height: round(calc(14px * var(--icon-scale)), 1px);
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
