<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import Play from '@lucide/svelte/icons/play'
  import Settings from '@lucide/svelte/icons/settings'
  import { SvelteSet } from 'svelte/reactivity'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import type { Diagnostic } from '../../lib/api/yaml'
  import { layout } from '../../lib/stores/layout.svelte'
  import { lintSettingsRoute } from '../../lib/stores/router.svelte'
  import { editorNav } from '../editor/active.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace } from '../workspace/workspace.svelte'
  import ProblemItem from './ProblemItem.svelte'
  import { groupTitles, lint } from './lint.svelte'

  // Lint every YAML file of the open folder, on demand. Files are read
  // from disk; what-if edits are checked live in their tabs.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

  if (!lint.loaded) lint.load()

  type Filter = 'all' | 'warning' | 'error'
  let severity = $state<Filter>('warning')
  let group = $state('all')
  const collapsed = new SvelteSet<string>()

  const rank = { error: 2, warning: 1, info: 0 }
  const min = $derived(severity === 'error' ? 2 : severity === 'warning' ? 1 : 0)

  function keep(d: Diagnostic): boolean {
    return rank[d.severity] >= min && (group === 'all' || (d.source || 'syntax') === group)
  }

  const shown = $derived(
    (lint.results ?? []).map((f) => ({ ...f, diagnostics: f.diagnostics.filter(keep) })).filter((f) => f.diagnostics.length > 0),
  )
  const totals = $derived.by(() => {
    const t = { error: 0, warning: 0, info: 0 }
    for (const f of lint.results ?? []) for (const d of f.diagnostics) t[d.severity]++
    return t
  })
  const groupItems = $derived([
    { value: 'all', label: 'All' },
    { value: 'syntax', label: 'Syntax' },
    ...Object.entries(groupTitles).map(([value, label]) => ({ value, label })),
  ])

  function open(path: string, d: Diagnostic) {
    layout.openFile(path)
    editorNav.request(path, d.range.start.line, d.range.start.col)
  }
</script>

<div class="problems-view" class:inactive={!active}>
  <header>
    <div class="title">
      <h1>Workspace problems</h1>
      {#if lint.results}
        <p>
          {lint.checked} YAML files checked in {lint.durationMs < 1000 ? `${lint.durationMs} ms` : `${(lint.durationMs / 1000).toFixed(1)} s`} ·
          <span class="err">{totals.error} errors</span> ·
          <span class="warn">{totals.warning} warnings</span> ·
          {totals.info} infos
        </p>
      {:else}
        <p>Syntax, style, Kubernetes checks, removed APIs and schemas for every YAML file.</p>
      {/if}
    </div>
    <Button variant="ghost" size="sm" icon={Settings} onclick={() => layout.open(lintSettingsRoute)}>Settings</Button>
    <Button variant="primary" size="sm" icon={Play} disabled={!workspace.info?.open || lint.running} onclick={() => lint.runWorkspace()}>
      {lint.running ? 'Checking…' : lint.results ? 'Check again' : 'Lint workspace'}
    </Button>
  </header>

  {#if !workspace.info?.open}
    <EmptyState icon={FolderOpen} title="No folder open" description="Open a repository folder to lint its YAML." />
  {:else if lint.error}
    <p class="error">{lint.error}</p>
  {:else if lint.results}
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
      <SegmentedControl label="Kind of check" options={groupItems} bind:value={group} />
    </div>
    {#if shown.length === 0}
      <p class="ok"><CircleCheck size={14} strokeWidth={1.75} /> Nothing at this level.</p>
    {:else}
      <div class="files">
        {#each shown as f (f.path)}
          {@const isOpen = !collapsed.has(f.path)}
          <section>
            <button
              type="button"
              class="file"
              aria-expanded={isOpen}
              onclick={() => (isOpen ? collapsed.add(f.path) : collapsed.delete(f.path))}
            >
              {#if isOpen}<ChevronDown size={14} />{:else}<ChevronRight size={14} />{/if}
              <FileIcon file={workspace.byPath.get(f.path) ?? { lang: 'yaml', type: f.type }} />
              <span class="name">{f.path}</span>
              <span class="count">{f.diagnostics.length}</span>
            </button>
            {#if isOpen}
              <ul>
                {#each f.diagnostics as d, i (i)}
                  <ProblemItem
                    severity={d.severity}
                    message={d.message}
                    hint={d.hint}
                    why={d.why}
                    code={d.code}
                    where="{d.range.start.line}:{d.range.start.col}"
                    onjump={() => open(f.path, d)}
                  />
                {/each}
              </ul>
            {/if}
          </section>
        {/each}
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
  header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-4) var(--s-5);
    border-bottom: 1px solid var(--border);
  }
  .title {
    flex: 1;
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  header p {
    margin: 2px 0 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .err {
    color: var(--err);
  }
  .warn {
    color: var(--warn);
  }
  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-3);
    padding: var(--s-3) var(--s-5);
    border-bottom: 1px solid var(--border);
  }
  .files {
    flex: 1;
    overflow: auto;
    padding: var(--s-2) var(--s-3);
  }
  .file {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .file:hover {
    background: var(--bg-2);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
  }
  .count {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  ul {
    margin: 0 0 var(--s-2);
    padding: 0 0 0 var(--s-5);
  }
  .ok,
  .error {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-4) var(--s-5);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .ok :global(svg) {
    color: var(--ok);
  }
  .error {
    color: var(--err);
  }
</style>
