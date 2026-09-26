<script lang="ts">
  import FileSearch from '@lucide/svelte/icons/file-search'
  import Play from '@lucide/svelte/icons/play'
  import Button from '../../lib/components/Button.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import type { QueryHit } from '../../lib/api/compare'
  import { layout } from '../../lib/stores/layout.svelte'
  import { editorNav } from '../editor/active.svelte'
  import { charts, helmNav } from '../helm/helm.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace } from '../workspace/workspace.svelte'
  import { queries as q } from './compare.svelte'

  // jq over YAML: the whole workspace, one file (with its what-if
  // edits) or a Helm render. Results jump to where they are.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

  const examples = [
    '.spec.template.spec.containers[].image',
    'select(.kind == "Deployment") | .spec.replicas',
    '.. | .image? | strings',
    'select(.kind == "Ingress") | .spec.rules[].host',
    '.metadata.labels',
  ]

  const chartItems = $derived(charts.list.map((c) => ({ value: c.path, label: c.path === '.' ? c.name : `${c.name} (${c.path})` })))
  const profileItems = $derived([
    { value: '', label: 'Chart defaults' },
    ...Object.keys(charts.byPath(q.chart || '.')?.profiles ?? {}).map((p) => ({ value: p, label: p })),
  ])
  const canRun = $derived(
    !!workspace.info?.open && (q.scope === 'workspace' || (q.scope === 'file' && !!q.path) || (q.scope === 'helm' && charts.list.length > 0)),
  )

  $effect(() => {
    if (q.scope === 'helm' && !q.chart && charts.list.length) q.chart = charts.list[0].path
  })

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (canRun) q.run()
    }
  }

  function open(h: QueryHit) {
    const line = h.range?.start.line ?? 1
    if (q.scope === 'helm') {
      helmNav.request(q.chart || '.', line)
      return
    }
    const file = h.file ?? q.path
    layout.openFile(file)
    editorNav.request(file, line, h.range?.start.col ?? 1)
  }
</script>

<div class="query" class:inactive={!active}>
  <header>
    <div class="expr">
      <textarea
        bind:value={q.expr}
        rows="1"
        spellcheck="false"
        aria-label="jq expression"
        placeholder=".spec.template.spec.containers[].image"
        {onkeydown}
      ></textarea>
      <Button variant="primary" size="sm" icon={Play} disabled={!canRun || q.running} onclick={() => q.run()}>
        {q.running ? 'Running…' : 'Run'}
      </Button>
    </div>
    <div class="scope">
      <SegmentedControl
        label="Search in"
        options={[
          { value: 'workspace', label: 'Workspace' },
          { value: 'file', label: 'File' },
          { value: 'helm', label: 'Helm render' },
        ]}
        bind:value={q.scope}
      />
      {#if q.scope === 'file'}
        <button type="button" class="file" onclick={() => workspace.pickFile('Choose a file to query', (p) => (q.path = p))}>
          {#if q.path}
            <FileIcon file={workspace.byPath.get(q.path) ?? { lang: 'yaml' }} />
            <span>{q.path}</span>
          {:else}
            <FileSearch size={14} strokeWidth={1.75} /> <span class="muted">Choose a file…</span>
          {/if}
        </button>
      {:else if q.scope === 'helm'}
        {#if chartItems.length}
          <Select items={chartItems} value={q.chart || '.'} label="Chart" onchange={(v) => ((q.chart = v), (q.profile = ''))} />
          <Select items={profileItems} value={q.profile} label="Profile" onchange={(v) => (q.profile = v)} />
        {:else}
          <span class="muted">No charts in this folder</span>
        {/if}
      {/if}
      <span class="hint">Each document is one input · Enter runs · <a href="https://jqlang.org/manual/" target="_blank" rel="noreferrer">jq manual</a></span>
    </div>
    <div class="examples">
      {#each examples as ex (ex)}
        <button type="button" class="chip" onclick={() => ((q.expr = ex), canRun && q.run())}>{ex}</button>
      {/each}
    </div>
  </header>

  <div class="results">
    {#if q.error}
      <p class="error">{q.error}</p>
    {:else if q.results}
      <p class="count">
        {q.results.length}{q.truncated ? '+' : ''} result{q.results.length === 1 ? '' : 's'}
        {#if q.truncated}(stopped at 500){/if}
        {#if q.timedOut}(timed out){/if}
      </p>
      <ul>
        {#each q.results as h, i (i)}
          <li>
            <button type="button" class="hit" onclick={() => open(h)}>
              <span class="where">
                {#if h.file}<FileIcon file={workspace.byPath.get(h.file) ?? { lang: 'yaml' }} />{/if}
                <span class="loc">{h.file ?? (q.scope === 'helm' ? 'rendered' : q.path)}:{h.range?.start.line ?? '?'}</span>
                {#if h.path}<span class="jpath">{h.path}</span>{/if}
              </span>
              <pre class="value">{h.value.split('\n').slice(0, 8).join('\n')}{h.value.split('\n').length > 8 ? '\n…' : ''}</pre>
            </button>
          </li>
        {:else}
          <li class="muted pad">No results.</li>
        {/each}
      </ul>
    {:else}
      <p class="muted pad">Run a jq expression over your YAML. Path expressions jump to the exact line; computed values point at their document.</p>
    {/if}
  </div>
</div>

<style>
  .query {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  header {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  .expr {
    display: flex;
    gap: var(--s-2);
    align-items: flex-start;
  }
  textarea {
    flex: 1;
    min-height: var(--control-h);
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    outline: none;
    resize: vertical;
    field-sizing: content;
  }
  textarea:focus {
    border-color: var(--accent);
  }
  .scope {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .file {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    max-width: 360px;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    overflow: hidden;
    white-space: nowrap;
  }
  .hint {
    margin-left: auto;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .hint a {
    color: var(--accent);
  }
  .examples {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
  }
  .chip {
    padding: 2px var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .chip:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  .results {
    flex: 1;
    overflow: auto;
    padding: var(--s-2) var(--s-3);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .count {
    margin: var(--s-1) var(--s-2) var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .hit {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    width: 100%;
    padding: var(--s-2);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .hit:hover {
    background: var(--bg-2);
  }
  .where {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    font-size: var(--fs-sm);
  }
  .loc {
    font-family: var(--font-mono);
    color: var(--fg-1);
    white-space: nowrap;
  }
  .jpath {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .value {
    margin: 0;
    padding-left: var(--s-5);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--syn-string);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .muted {
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-3);
    font-size: var(--fs-sm);
  }
  .error {
    padding: var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--err);
    white-space: pre-wrap;
  }
</style>
