<script lang="ts">
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import { hueVar, viewHue, viewIcon } from '../../lib/tools'
  import ChartBar from '@lucide/svelte/icons/chart-bar'
  import Copy from '@lucide/svelte/icons/copy'
  import FileSearch from '@lucide/svelte/icons/file-search'
  import Lightbulb from '@lucide/svelte/icons/lightbulb'
  import Play from '@lucide/svelte/icons/play'
  import Sparkles from '@lucide/svelte/icons/sparkles'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import Chip from '../../lib/components/Chip.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import type { QueryHit } from '../../lib/api/compare'
  import { layout } from '../../lib/stores/layout.svelte'
  import { narrow } from '../../lib/stores/narrow.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import { editorNav } from '../editor/active.svelte'
  import { charts, helmNav } from '../helm/helm.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace } from '../workspace/workspace.svelte'
  import { queries as q } from './compare.svelte'
  import { queryHint } from './queryHint'

  // jq over YAML: the whole workspace, one file (with its what-if
  // edits) or a Helm render. Results jump to where they are; beside
  // them, how many each file has and a hint for what was found.
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

  const fileOf = (h: QueryHit) => h.file ?? (q.scope === 'helm' ? 'rendered' : q.path)

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

  const results = $derived(q.results ?? [])
  // Most first; a file's meter is its share of the file with the most.
  const byFile = $derived.by(() => {
    const n = new Map<string, number>()
    for (const h of results) n.set(fileOf(h), (n.get(fileOf(h)) ?? 0) + 1)
    return [...n].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
  })
  const most = $derived(byFile[0]?.[1] ?? 1)
  const shownFiles = 8
  const hint = $derived(queryHint(q.expr, results))

  const took = (ms: number) => (ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`)
  const clip = (v: string) => {
    const lines = v.split('\n')
    return lines.length > 8 ? lines.slice(0, 8).join('\n') + '\n…' : v
  }
</script>

<div class="query hued" class:inactive={!active} style:--tool={hueVar(viewHue.query)}>
  <PanelHeader icon={viewIcon.query} title="Query">
    {#snippet meta()}jq (gojq) over YAML documents{/snippet}
    {#snippet actions()}
      <span class="lbl">Search in</span>
      <SegmentedControl
        label="Search in"
        options={[
          { value: 'workspace', label: 'Workspace' },
          { value: 'file', label: 'File' },
          { value: 'helm', label: 'Helm render' },
        ]}
        bind:value={q.scope}
      />
    {/snippet}
  </PanelHeader>

  <div class="ask">
    <div class="expr">
      <label class="box">
        <span class="jq">jq</span>
        <textarea bind:value={q.expr} rows="1" spellcheck="false" aria-label="jq expression" placeholder=".spec.template.spec.containers[].image" {onkeydown}></textarea>
      </label>
      <Button variant="primary" icon={Play} tone="run" disabled={!canRun || q.running} title={formatShortcut('Mod+Enter')} onclick={() => q.run()}>
        {q.running ? 'Running…' : 'Run'}
      </Button>
    </div>
    {#if q.scope !== 'workspace'}
      <div class="scope">
        {#if q.scope === 'file'}
          <button type="button" class="file" onclick={() => workspace.pickFile('Choose a file to query', (p) => (q.path = p))}>
            {#if q.path}
              <FileIcon file={workspace.byPath.get(q.path) ?? { lang: 'yaml' }} />
              <span>{q.path}</span>
            {:else}
              <FileSearch size={14} /> <span class="muted">Choose a file…</span>
            {/if}
          </button>
        {:else if chartItems.length}
          <Select items={chartItems} value={q.chart || '.'} label="Chart" onchange={(v) => ((q.chart = v), (q.profile = ''))} />
          <Select items={profileItems} value={q.profile} label="Profile" onchange={(v) => (q.profile = v)} />
        {:else}
          <span class="muted">No charts in this folder</span>
        {/if}
      </div>
    {/if}
    <div class="examples">
      <span class="try">Try</span>
      {#each examples as ex (ex)}
        <Chip mono onclick={() => ((q.expr = ex), canRun && q.run())}><Sparkles size={12} />{ex}</Chip>
      {/each}
      <span class="hint">Each document is one input · Enter runs · <a href="https://jqlang.org/manual/" target="_blank" rel="noreferrer">jq manual</a></span>
    </div>
  </div>

  {#snippet hintCard()}
    <p class="found"><Lightbulb size={14} />{hint}</p>
  {/snippet}

  <div class="body">
    <section class="results" aria-label="Results">
      {#if q.error}
        <ErrorNote message={q.error} />
      {:else if q.results}
        <div class="count">
          <span class="n">{results.length}{q.truncated ? '+' : ''} result{results.length === 1 ? '' : 's'}</span>
          <span class="of">
            in {byFile.length} file{byFile.length === 1 ? '' : 's'} · {took(q.took)}
            {#if q.truncated}· stopped at 500{/if}
            {#if q.timedOut}· timed out{/if}
          </span>
          {#if results.length}
            <Button size="sm" icon={Copy} tone="copy" onclick={() => copyText(results.map((h) => h.value).join('\n'), 'Results')}>Copy all</Button>
          {/if}
        </div>
        {#if hint && narrow.current}{@render hintCard()}{/if}
        <ul class="grid">
          {#each results as h, i (i)}
            <li class="hit">
              <button type="button" class="open" onclick={() => open(h)}>
                <span class="where">
                  {#if h.file}<FileIcon file={workspace.byPath.get(h.file) ?? { lang: 'yaml' }} />{/if}
                  <span class="loc" title={h.path}>{fileOf(h)}</span>
                  <Badge mono>:{h.range?.start.line ?? '?'}</Badge>
                </span>
                <span class="value">{clip(h.value)}</span>
              </button>
              <span class="copy"><IconButton icon={Copy} tone="copy" size="sm" label="Copy value" onclick={() => copyText(h.value, 'Value')} /></span>
            </li>
          {:else}
            <li class="muted pad">No results.</li>
          {/each}
        </ul>
      {:else}
        <p class="muted pad">Run a jq expression over your YAML. Path expressions jump to the exact line; computed values point at their document.</p>
      {/if}
    </section>

    {#if !narrow.current && results.length}
      <aside class="by-file" aria-label="Results by file">
        <div class="pane-title"><ChartBar size={14} />Results by file</div>
        <div class="files">
          {#each byFile.slice(0, shownFiles) as [f, n] (f)}
            <div class="file-row">
              <span class="name"><span class="path" title={f}>{f}</span><b>{n}</b></span>
              <span class="meter" aria-hidden="true"><i style:width="{(n / most) * 100}%"></i></span>
            </div>
          {/each}
          {#if byFile.length > shownFiles}<p class="more">and {byFile.length - shownFiles} more</p>{/if}
          {#if hint}{@render hintCard()}{/if}
        </div>
      </aside>
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
  .lbl {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .ask {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding: var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  .expr {
    display: flex;
    gap: var(--s-3);
    align-items: stretch;
  }
  .expr > :global(.btn) {
    height: auto;
    min-height: calc(var(--control-h) + var(--s-4));
    padding: 0 var(--s-5);
    border-radius: var(--r-lg);
  }
  /* The expression box: "jq" in the view's colour, then the expression,
     which grows with what is typed. */
  .box {
    flex: 1;
    display: flex;
    align-items: center;
    gap: var(--s-3);
    min-height: calc(var(--control-h) + var(--s-4));
    padding: var(--s-2) var(--s-4);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-lg);
  }
  .box:focus-within {
    border-color: var(--tool-accent);
    box-shadow: 0 0 0 1px var(--tool-accent);
  }
  .jq {
    font-family: var(--font-mono);
    font-size: var(--fs-lg);
    font-weight: var(--fw-bold);
    color: var(--tool-accent);
  }
  textarea {
    flex: 1;
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-lg);
    color: var(--syn-key);
    background: none;
    border: none;
    outline: none;
    resize: none;
    field-sizing: content;
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
    border-radius: var(--r-md);
    overflow: hidden;
    white-space: nowrap;
  }
  .examples {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .examples :global(.chip svg) {
    color: var(--tool-accent);
  }
  .try {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .hint {
    margin-left: auto;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .hint a {
    color: var(--tool-accent);
  }
  .body {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  .results {
    flex: 1;
    min-width: 0;
    overflow: auto;
    padding: var(--s-4);
  }
  .count {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  .n {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .of {
    flex: 1;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 360px), 1fr));
    gap: var(--s-3);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  /* A result: its file and line on top, the value under it; the card
     opens it, the copy button in its corner copies the value. */
  .hit {
    position: relative;
    min-width: 0;
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
  }
  .hit:hover {
    border-color: var(--tool-line);
  }
  .open {
    display: flex;
    flex-direction: column;
    width: 100%;
    padding: 0;
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .where {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-2) calc(var(--control-h-sm) + var(--s-3)) var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .loc {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .copy {
    position: absolute;
    top: 2px;
    right: var(--s-1);
  }
  .value {
    display: block;
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--syn-string);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .by-file {
    flex: none;
    display: flex;
    flex-direction: column;
    width: 300px;
    min-height: 0;
    border-left: 1px solid var(--border);
    background: var(--bg-1);
  }
  .pane-title {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: calc(var(--control-h) + var(--s-2));
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    border-bottom: 1px solid var(--border);
  }
  .pane-title :global(svg) {
    color: var(--tool-accent);
  }
  .files {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding: var(--s-3);
    overflow: auto;
  }
  .file-row {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .name {
    display: flex;
    gap: var(--s-2);
    font-size: var(--fs-sm);
  }
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
  }
  .meter {
    height: var(--s-2);
    background: var(--bg-3);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .meter i {
    display: block;
    height: 100%;
    background: var(--tool-accent);
    border-radius: inherit;
  }
  .more {
    margin: 0;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .found {
    display: flex;
    gap: var(--s-2);
    margin: 0 0 var(--s-3);
    padding: var(--s-3);
    font-size: var(--fs-xs);
    line-height: 1.4;
    color: var(--fg-1);
    background: color-mix(in srgb, var(--warn) var(--wash), var(--bg-1));
    border: 1px solid color-mix(in srgb, var(--warn) 45%, var(--border));
    border-radius: var(--r-lg);
  }
  .files .found {
    margin: var(--s-2) 0 0;
  }
  .found :global(svg) {
    flex: none;
    color: var(--warn);
  }
  .muted {
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-3);
    font-size: var(--fs-sm);
  }
</style>
