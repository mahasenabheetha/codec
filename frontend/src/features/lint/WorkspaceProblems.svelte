<script lang="ts">
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import { toolCommands } from '../shared/tool.svelte'
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import { hueVar, viewHue, type Hue } from '../../lib/tools'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import Info from '@lucide/svelte/icons/info'
  import Play from '@lucide/svelte/icons/play'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal'
  import { SvelteSet } from 'svelte/reactivity'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import Card from '../../lib/components/Card.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import VirtualList from '../../lib/components/VirtualList.svelte'
  import type { Diagnostic } from '../../lib/api/yaml'
  import { layout } from '../../lib/stores/layout.svelte'
  import { settingsRoute } from '../../lib/stores/router.svelte'
  import { editorNav } from '../editor/active.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { isSpecific, lookOf } from '../workspace/filetypes'
  import { workspace } from '../workspace/workspace.svelte'
  import ProblemItem from './ProblemItem.svelte'
  import { groupTitles, lint } from './lint.svelte'

  // Lint every YAML file of the open folder, on demand. Files are read
  // from disk; what-if edits are checked live in their tabs.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

  // In the palette while this tab is shown (3.10.2).
  toolCommands(
    () => active,
    () => [{ id: 'problems.view.run', title: 'Problems: Check the workspace again', group: 'Problems', run: () => !lint.running && lint.runWorkspace() }],
  )

  if (!lint.loaded) lint.load()

  type Filter = 'all' | 'warning' | 'error'
  let severity = $state<Filter>('warning')
  let group = $state('all')
  let text = $state('')
  const collapsed = new SvelteSet<string>()

  const rank = { error: 2, warning: 1, info: 0 }
  const min = $derived(severity === 'error' ? 2 : severity === 'warning' ? 1 : 0)
  const needle = $derived(text.trim().toLowerCase())

  const groupOf = (d: Diagnostic) => d.source || 'syntax'
  function keep(path: string, d: Diagnostic): boolean {
    return (
      rank[d.severity] >= min &&
      (group === 'all' || groupOf(d) === group) &&
      (!needle || path.toLowerCase().includes(needle) || d.code.toLowerCase().includes(needle))
    )
  }

  const shown = $derived(
    (lint.results ?? []).map((f) => ({ ...f, diagnostics: f.diagnostics.filter((d) => keep(f.path, d)) })).filter((f) => f.diagnostics.length > 0),
  )
  // One row per file and per problem of an open file, so the list can
  // be windowed: a big folder has thousands of findings.
  type Row = { key: string; f: (typeof shown)[number]; d?: Diagnostic; first?: boolean; last?: boolean }
  const rows = $derived.by(() => {
    const out: Row[] = []
    shown.forEach((f, n) => {
      out.push({ key: `f:${f.path}`, f, first: n === 0, last: collapsed.has(f.path) })
      if (collapsed.has(f.path)) return
      f.diagnostics.forEach((d, i) => out.push({ key: `p:${f.path}#${i}`, f, d, last: i === f.diagnostics.length - 1 }))
    })
    return out
  })
  const totals = $derived(lint.totals)
  const all = $derived(totals.error + totals.warning + totals.info)
  const groupItems = $derived([
    { value: 'all', label: 'All checks' },
    { value: 'syntax', label: 'Syntax' },
    ...Object.entries(groupTitles).map(([value, label]) => ({ value, label })),
  ])

  // The summary over everything found, whatever the filters show.
  const groupHue: Record<string, Hue> = { syntax: 'orchid', style: 'peach', kubernetes: 'indigo', deprecation: 'teal', schema: 'cyan' }
  const byGroup = $derived.by(() => {
    const n = new Map<string, number>()
    for (const f of lint.results ?? []) for (const d of f.diagnostics) n.set(groupOf(d), (n.get(groupOf(d)) ?? 0) + 1)
    const most = Math.max(1, ...n.values())
    return groupItems
      .filter((g) => g.value !== 'all' && n.has(g.value))
      .map((g) => ({ ...g, count: n.get(g.value)!, share: n.get(g.value)! / most, hue: groupHue[g.value] ?? viewHue.problems }))
  })
  const worst = $derived(
    [...(lint.results ?? [])]
      .filter((f) => f.diagnostics.length)
      .sort((a, b) => b.diagnostics.length - a.diagnostics.length || a.path.localeCompare(b.path))
      .slice(0, 3),
  )

  const fileOf = (path: string, type: string) => workspace.byPath.get(path) ?? { lang: 'yaml', type }
  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`
  function countOf(ds: Diagnostic[]): { text: string; tone: 'err' | 'warn' | 'info' } {
    const e = ds.filter((d) => d.severity === 'error').length
    const w = ds.filter((d) => d.severity === 'warning').length
    const i = ds.length - e - w
    const parts = [e && plural(e, 'error'), w && plural(w, 'warning'), i && plural(i, 'info')].filter(Boolean)
    return { text: parts.join(' · '), tone: e ? 'err' : w ? 'warn' : 'info' }
  }
  const took = (ms: number) => (ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`)

  function open(path: string, d: Diagnostic) {
    layout.openFile(path)
    editorNav.request(path, d.range.start.line, d.range.start.col)
  }
</script>

<div class="problems-view hued" class:inactive={!active} style:--tool={hueVar(viewHue.problems)}>
  <PanelHeader icon={TriangleAlert} title="Workspace problems">
    {#snippet meta()}
      {#if lint.results}
        <span>{lint.checked} YAML files checked in {took(lint.durationMs)}</span>
      {:else}
        <span>Syntax, style, Kubernetes checks, removed APIs and schemas for every YAML file.</span>
      {/if}
    {/snippet}
    {#snippet actions()}
      <Button icon={SlidersHorizontal} tone="open" onclick={() => layout.open(settingsRoute)}>Settings</Button>
      <Button variant="primary" icon={lint.results ? RefreshCw : Play} disabled={!workspace.info?.open || lint.running} onclick={() => lint.runWorkspace()}>
        {lint.running ? 'Checking…' : lint.results ? 'Check again' : 'Lint workspace'}
      </Button>
    {/snippet}
  </PanelHeader>

  {#if !workspace.info?.open}
    <EmptyState icon={FolderOpen} title="No folder open" description="Open a repository folder to lint its YAML." />
  {:else if lint.error}
    <ErrorNote message={lint.error} retry={() => lint.runWorkspace()} />
  {:else if lint.results}
    {#if all}
      <div class="summary">
        <Card tag="section" class="sum" aria-label="By severity">
          <div class="sum-head"><span class="sum-title">By severity</span><span class="dim">{all} in all</span></div>
          <div class="bar" aria-hidden="true">
            {#if totals.error}<span class="err" style:flex={totals.error}></span>{/if}
            {#if totals.warning}<span class="warn" style:flex={totals.warning}></span>{/if}
            {#if totals.info}<span class="info" style:flex={totals.info}></span>{/if}
          </div>
          <div class="sev">
            <div><span class="sev-name"><CircleX size={13} />Errors</span><span class="big err">{totals.error}</span></div>
            <div><span class="sev-name"><TriangleAlert size={13} />Warnings</span><span class="big warn">{totals.warning}</span></div>
            <div><span class="sev-name"><Info size={13} />Infos</span><span class="big info">{totals.info}</span></div>
          </div>
        </Card>
        <Card tag="section" class="sum" aria-label="By kind of check">
          <span class="sum-title">By kind of check</span>
          {#each byGroup as g (g.value)}
            <div class="meter-row hued" style:--tool={hueVar(g.hue)}>
              <span class="dim">{g.label}</span>
              <span class="meter" aria-hidden="true"><i style:width="{Math.round(g.share * 100)}%"></i></span>
              <span class="n">{g.count}</span>
            </div>
          {/each}
        </Card>
        <Card tag="section" class="sum" aria-label="Most problems">
          <span class="sum-title">Most problems</span>
          {#each worst as f (f.path)}
            <div class="worst">
              <FileIcon file={fileOf(f.path, f.type)} />
              <span class="path" title={f.path}>{f.path}</span>
              <Badge>{f.diagnostics.length}</Badge>
            </div>
          {/each}
        </Card>
      </div>
    {/if}
    <div class="filters">
      <SegmentedControl
        label="Minimum severity"
        options={[
          { value: 'error', label: 'Errors' },
          { value: 'warning', label: 'Warnings +' },
          { value: 'all', label: 'All' },
        ]}
        bind:value={severity}
      />
      <SegmentedControl soft label="Kind of check" options={groupItems} bind:value={group} />
      <span class="grow"></span>
      <SearchInput bind:value={text} placeholder="Filter by path or rule" label="Filter problems by path or rule" />
    </div>
    {#if shown.length === 0}
      <p class="ok"><CircleCheck size={14} /> {needle ? 'Nothing matches the filter.' : 'Nothing at this level.'}</p>
    {:else}
      <div class="files">
        <VirtualList count={rows.length} key={(i) => rows[i].key} estimate={44}>
          {#snippet row(i)}
            {@const r = rows[i]}
            <div class="gap" class:first={r.first || !!r.d}>
              {#if r.d === undefined}
                {@const isOpen = !collapsed.has(r.f.path)}
                {@const c = countOf(r.f.diagnostics)}
                {@const kind = r.f.type || workspace.byPath.get(r.f.path)?.type || ''}
                <button
                  type="button"
                  class="file"
                  class:alone={r.last}
                  aria-expanded={isOpen}
                  onclick={() => (isOpen ? collapsed.add(r.f.path) : collapsed.delete(r.f.path))}
                >
                  {#if isOpen}<ChevronDown size={14} />{:else}<ChevronRight size={14} />{/if}
                  <FileIcon file={fileOf(r.f.path, r.f.type)} />
                  <span class="name">{r.f.path}</span>
                  {#if isSpecific(kind)}<Badge>{lookOf(kind).title}</Badge>{/if}
                  <span class="grow"></span>
                  <Badge tone={c.tone}>{c.text}</Badge>
                </button>
              {:else}
                {@const d = r.d}
                <div class="problem" class:last={r.last}>
                  <ProblemItem
                    tag="div"
                    inline
                    severity={d.severity}
                    message={d.message}
                    hint={d.hint}
                    why={d.why}
                    code={d.code}
                    where="{d.range.start.line}:{d.range.start.col}"
                    onjump={() => open(r.f.path, d)}
                  />
                </div>
              {/if}
            </div>
          {/snippet}
        </VirtualList>
      </div>
    {/if}
  {/if}
</div>

<style>
  .problems-view {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .dim {
    color: var(--fg-1);
  }

  /* The summary: by severity, by kind of check, the files with most. */
  .summary {
    flex: none;
    display: grid;
    grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) minmax(0, 1fr);
    gap: var(--s-3);
    padding: var(--s-4) var(--s-4) var(--s-1);
  }
  .summary > :global(.card.sum) {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    font-size: var(--fs-sm);
  }
  .sum-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .sum-head .dim {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .sum-title {
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
  }
  .bar {
    display: flex;
    gap: 2px;
    height: var(--s-3);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .bar .err {
    background: var(--err);
  }
  .bar .warn {
    background: var(--warn);
  }
  .bar .info {
    background: var(--info);
  }
  .sev {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--s-2);
  }
  .sev > div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .sev-name {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    color: var(--fg-1);
  }
  .big {
    font-size: var(--fs-3xl);
    font-weight: var(--fw-semibold);
    line-height: 1.1;
  }
  .err,
  .sev > div:nth-child(1) .sev-name :global(svg) {
    color: var(--err);
  }
  .warn,
  .sev > div:nth-child(2) .sev-name :global(svg) {
    color: var(--warn);
  }
  .info,
  .sev > div:nth-child(3) .sev-name :global(svg) {
    color: var(--info);
  }
  .meter-row {
    display: grid;
    grid-template-columns: minmax(0, 7em) minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--s-2);
  }
  .meter {
    height: var(--s-2);
    border-radius: var(--r-full);
    background: var(--bg-4);
    overflow: hidden;
  }
  .meter i {
    display: block;
    height: 100%;
    border-radius: var(--r-full);
    background: var(--tool-accent);
  }
  .n {
    font-weight: var(--fw-semibold);
    text-align: right;
  }
  .worst {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
  }
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
  }

  .filters {
    flex: none;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-3);
    padding: var(--s-3) var(--s-4);
  }
  .filters > :global(.search) {
    flex: 0 1 260px;
  }
  .filters > :global(.search input) {
    width: 100%;
  }
  .grow {
    flex: 1;
  }

  /* Each file is a pane: its header, then its problems. */
  .files {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 0 var(--s-4) var(--s-4);
  }
  .gap {
    padding-top: var(--s-3);
  }
  .gap.first {
    padding-top: 0;
  }
  .file {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    min-height: calc(var(--control-h) + var(--s-2));
    padding: 0 var(--s-3);
    font-size: var(--fs-md);
    text-align: left;
    color: var(--fg-0);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg) var(--r-lg) 0 0;
  }
  .file.alone {
    border-radius: var(--r-lg);
  }
  .file:hover {
    background: var(--bg-3);
  }
  .file > :global(svg:first-child) {
    color: var(--fg-2);
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-weight: var(--fw-medium);
  }
  .problem {
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-top: none;
  }
  .problem.last {
    border-radius: 0 0 var(--r-lg) var(--r-lg);
    overflow: hidden;
  }
  .problem.last > :global(.item) {
    border-bottom: none;
  }
  .ok {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
  }
  .ok,
  .ok :global(svg) {
    color: var(--ok);
  }
  @media (max-width: 900px) {
    .summary {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
