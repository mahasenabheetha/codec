<script lang="ts">
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import Columns3 from '@lucide/svelte/icons/columns-3'
  import Copy from '@lucide/svelte/icons/copy'
  import Sheet from '@lucide/svelte/icons/sheet'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { untrack } from 'svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { convertTable, parseTable, tableFromJSON, type Delimiter, type HeaderMode, type TableColumn, type TableFormat, type TableInfo } from '../../lib/api/table'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextOutput from '../encode/TextOutput.svelte'
  import { liveJob } from '../json/doc.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job, TextJob } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import TableView from './TableView.svelte'

  // CSV and TSV: read into a table (the delimiter and header row guessed
  // unless chosen), shown with types and counts per column, converted to
  // JSON, YAML, Markdown or the other delimiter; and a JSON array back to
  // CSV. Pasted text is never stored (19).
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type View = 'table' | 'convert' | 'from-json'
  let view = $state<View>('table')
  let delimiter = $state<Delimiter>('')
  let header = $state<HeaderMode>('auto')
  let to = $state<TableFormat>('json')
  let typed = $state(true)
  let jsonTo = $state<'csv' | 'tsv'>('csv')
  let filter = $state('')
  let summary = $state(false)
  let shown = $state(0)

  const doc = new TextJob<never>()
  const json = new TextJob<never>()
  const parsed = new Job<TableInfo & { rows: string[][] }>()
  const converted = new Job<TableInfo & { output: string }>()
  const fromJSON = new Job<{ output: string; columns: TableColumn[]; total: number }>()
  let editor = $state<CodeView>()

  const src = $derived(view === 'from-json' ? json : doc)
  const job = $derived(view === 'table' ? parsed : view === 'convert' ? converted : fromJSON)
  const info = $derived<TableInfo | null>(view === 'table' ? parsed.result : view === 'convert' ? converted.result : null)

  // CSV or TSV handed over by smart paste.
  $effect(() => {
    const v = handoff.take('text', 'table')
    if (v !== null)
      untrack(() => {
        view = 'table'
        delimiter = ''
        header = 'auto'
        doc.input = v
      })
  })

  liveJob(() => active && view === 'table', parsed, () => {
    const input = doc.input
    if (!input.trim()) return null
    const o = { delimiter, header }
    return () => parseTable(input, o)
  })
  liveJob(() => active && view === 'convert', converted, () => {
    const input = doc.input
    if (!input.trim()) return null
    const o = { delimiter, header, typed }
    const format = to
    return () => convertTable(input, format, o)
  })
  liveJob(() => active && view === 'from-json', fromJSON, () => {
    const input = json.input
    if (!input.trim()) return null
    const format = jsonTo
    return () => tableFromJSON(input, format)
  })

  const outText = $derived(view === 'convert' ? (converted.result?.output ?? null) : view === 'from-json' ? (fromJSON.result?.output ?? null) : null)
  const outLanguage = $derived(view === 'convert' && (to === 'json' || to === 'jsonl') ? 'json' : view === 'convert' && to === 'yaml' ? 'yaml' : 'text')

  const delimName: Record<string, string> = { ',': 'comma', '\t': 'tab', ';': 'semicolon', '|': 'pipe' }

  // The rows shown, as TSV with the header, for Copy in the table view.
  function rowsAsTSV(): string {
    const r = parsed.result
    if (!r) return ''
    const q = filter.trim().toLowerCase()
    const rows = q ? r.rows.filter((row) => row.some((v) => v.toLowerCase().includes(q))) : r.rows
    const cell = (v: string) => (/[\t"\r\n]/.test(v) ? `"${v.replaceAll('"', '""')}"` : v)
    const lines = rows.map((row) => row.map(cell).join('\t'))
    if (r.header) lines.unshift(r.columns.map((c) => cell(c.name)).join('\t'))
    return lines.join('\n') + '\n'
  }

  function run() {
    if (view === 'table') parsed.run(doc.input.trim() ? () => parseTable(doc.input, { delimiter, header }) : null)
    else if (view === 'convert') converted.run(doc.input.trim() ? () => convertTable(doc.input, to, { delimiter, header, typed }) : null)
    else fromJSON.run(json.input.trim() ? () => tableFromJSON(json.input, jsonTo) : null)
  }
  const copy = () => (view === 'table' ? copyText(rowsAsTSV(), 'Rows') : copyText(outText ?? ''))
  function clear() {
    src.clear()
    job.reset()
    editor?.focus()
  }
  // Convert to CSV or TSV, or JSON → CSV: the result becomes the table.
  function swap() {
    if (!outText) return
    if (view === 'from-json' || to === 'csv' || to === 'tsv') {
      doc.input = outText
      delimiter = ''
      view = 'table'
    } else if (to === 'json') {
      json.input = outText
      view = 'from-json'
    }
  }
  const canSwap = $derived(view === 'from-json' || (view === 'convert' && (to === 'csv' || to === 'tsv' || to === 'json')))
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  function sample() {
    view === 'from-json'
      ? (json.input = '[\n  { "name": "api", "replicas": 3, "ready": true },\n  { "name": "worker", "replicas": 1, "ready": false, "labels": { "tier": "jobs" } }\n]')
      : (doc.input =
          'id,name,team,joined,active,score\n1,Ann Lee,platform,2024-03-11,true,92.5\n2,"Smith, Bob",data,2023-11-02,false,78\n3,Chen Wu,platform,2025-01-20,true,\n4,Dana Ruiz,"web, mobile",2022-07-14,true,88.25\n5,Eli Park,data,2024-09-30,false,64\n')
  }

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolStatus(() => active, () => {
    const e = job.error
    if (e) return { text: e.line ? `${e.message} · line ${e.line}, col ${e.column}` : e.message, tone: 'err' }
    if (view === 'from-json' && fromJSON.result) {
      const r = fromJSON.result
      return { text: `${r.total.toLocaleString()} ${r.total === 1 ? 'row' : 'rows'} · ${r.columns.length} ${r.columns.length === 1 ? 'column' : 'columns'}`, tone: 'ok' }
    }
    if (info) {
      const rows = `${info.total.toLocaleString()} ${info.total === 1 ? 'row' : 'rows'}`
      const tone = info.ragged ? 'warn' : 'ok'
      return { text: `${rows} · ${info.columns.length} ${info.columns.length === 1 ? 'column' : 'columns'} · ${delimName[info.delimiter]}${info.ragged ? ` · ${info.ragged} uneven` : ''}`, tone }
    }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <SegmentedControl
      label="View"
      bind:value={view}
      options={[
        { value: 'table', label: 'Table' },
        { value: 'convert', label: 'Convert' },
        { value: 'from-json', label: 'JSON → CSV' },
      ]}
    />
    {#if view === 'from-json'}
      <Select
        label="Write as"
        bind:value={jsonTo}
        items={[
          { value: 'csv', label: 'CSV' },
          { value: 'tsv', label: 'TSV' },
        ]}
      />
    {:else}
      <Select
        label="Delimiter"
        bind:value={delimiter}
        items={[
          { value: '', label: 'Delimiter: guess' },
          { value: ',', label: 'Comma' },
          { value: '\t', label: 'Tab' },
          { value: ';', label: 'Semicolon' },
          { value: '|', label: 'Pipe' },
        ]}
      />
      <Select
        label="Header row"
        bind:value={header}
        items={[
          { value: 'auto', label: 'Header: guess' },
          { value: 'yes', label: 'Header row' },
          { value: 'no', label: 'No header' },
        ]}
      />
    {/if}
    {#if view === 'convert'}
      <Select
        label="Convert to"
        bind:value={to}
        items={[
          { value: 'json', label: 'JSON' },
          { value: 'jsonl', label: 'JSON Lines' },
          { value: 'yaml', label: 'YAML' },
          { value: 'markdown', label: 'Markdown' },
          { value: 'csv', label: 'CSV' },
          { value: 'tsv', label: 'TSV' },
        ]}
      />
      <Toggle label="Typed values" bind:checked={typed} disabled={to === 'markdown' || to === 'csv' || to === 'tsv'} />
    {/if}
  {/snippet}
  {#snippet input()}
    <InputPane
      session={src}
      bind:editor
      onclear={clear}
      language={view === 'from-json' ? 'auto' : 'text'}
      placeholder={view === 'from-json'
        ? 'Paste a JSON array of objects (or arrays, or JSON Lines): one column per key, nested values written as JSON'
        : 'Paste CSV or TSV: the delimiter and header row are guessed, quoted fields may hold the delimiter and line breaks'}
    />
  {/snippet}
  {#snippet output()}
    {#if view === 'table'}
      {@const r = parsed.result}
      <Pane title="Table">
        {#snippet meta()}
          {#if r}
            <span title="{r.delimGuessed ? 'Delimiter guessed' : 'Delimiter chosen'}; {r.header ? 'the first row names the columns' : 'no header row'}{r.headerGuessed ? ' (guessed)' : ''}"
              ><Badge>{delimName[r.delimiter]} · {r.header ? 'header' : 'no header'}</Badge></span
            >
            {#if r.ragged}<Badge tone="warn" icon={TriangleAlert}>{r.ragged} uneven {r.ragged === 1 ? 'row' : 'rows'}</Badge>{/if}
            <span>{filter.trim() ? `${shown.toLocaleString()} of ` : ''}{r.total.toLocaleString()} {r.total === 1 ? 'row' : 'rows'}{r.rows.length < r.total ? ` (first ${r.rows.length.toLocaleString()} shown)` : ''}</span>
          {/if}
          {#if parsed.busy}<span class="busy" aria-label="Working"></span>{/if}
        {/snippet}
        {#snippet actions()}
          {#if r}
            <SearchInput bind:value={filter} placeholder="Filter rows…" label="Filter rows" />
            <IconButton icon={Columns3} label="Column summary" size="sm" active={summary} onclick={() => (summary = !summary)} />
          {/if}
          <Button icon={Copy} tone="copy" variant="ghost" size="sm" title="Copy the rows shown as TSV (Alt+C)" disabled={!r?.rows.length} onclick={copy}>Copy</Button>
        {/snippet}
        {#if parsed.error}
          <ErrorBanner error={parsed.error} onjump={jump} />
        {:else if r && summary}
          <div class="summary" role="table" aria-label="Columns">
            <div class="srow shead" role="row">
              <span role="columnheader">Column</span><span role="columnheader">Type</span><span role="columnheader">Filled</span><span role="columnheader">Empty</span><span role="columnheader">Distinct</span>
            </div>
            {#each r.columns as c, i (i)}
              <div class="srow" role="row">
                <span class="cname" role="cell">{c.name}</span>
                <span class="ctype {c.type}" role="cell">{c.type === 'empty' ? 'no values' : c.type}</span>
                <span role="cell">{c.filled.toLocaleString()}</span>
                <span role="cell" class:dim={!c.empty}>{c.empty.toLocaleString()}</span>
                <span role="cell">{c.distinct.toLocaleString()}{c.distinctMore ? '+' : ''}</span>
              </div>
            {/each}
          </div>
        {:else if r && r.columns.length}
          <TableView columns={r.columns} rows={r.rows} {filter} bind:shown />
        {:else if r}
          <EmptyState icon={Sheet} title="No rows" description="The input holds no fields." />
        {:else}
          <EmptyState
            icon={Sheet}
            title="The table appears here"
            description="Sort by a column, filter the rows, and see each column's type (integer, number, boolean, date or text) and how many values are filled, empty and distinct. Double-click a cell to copy it."
          >
            <Button size="sm" onclick={sample}>Try a sample</Button>
          </EmptyState>
        {/if}
      </Pane>
    {:else}
      <TextOutput
        output={outText}
        error={job.error}
        busy={job.busy}
        language={outLanguage}
        emptyIcon={view === 'from-json' ? Sheet : ArrowLeftRight}
        emptyTitle={view === 'from-json' ? 'CSV appears here' : 'The conversion appears here'}
        emptyDescription={view === 'from-json'
          ? 'One column per key in the order first seen; nested objects and arrays are written as JSON, null as an empty cell.'
          : 'Typed values give numbers, booleans and null by each column’s type; ZIP codes such as 007 stay text. Markdown right-aligns number columns.'}
        onsample={sample}
        onswap={canSwap ? swap : undefined}
        onjump={jump}
      >
        {#snippet badges()}
          {#if view === 'convert' && converted.result}
            {@const r = converted.result}
            <Badge>{r.total.toLocaleString()} {r.total === 1 ? 'row' : 'rows'}</Badge>
            {#if r.ragged}<Badge tone="warn" icon={TriangleAlert}>{r.ragged} uneven</Badge>{/if}
          {:else if view === 'from-json' && fromJSON.result}
            <Badge>{fromJSON.result.columns.length} {fromJSON.result.columns.length === 1 ? 'column' : 'columns'}</Badge>
          {/if}
        {/snippet}
      </TextOutput>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .summary {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-2);
    font-size: var(--fs-sm);
  }
  .srow {
    display: grid;
    grid-template-columns: minmax(120px, 2fr) 1fr 1fr 1fr 1fr;
    gap: var(--s-2);
    padding: 4px var(--s-2);
    border-radius: var(--r-sm);
  }
  .srow:not(.shead):hover {
    background: var(--bg-3);
  }
  .srow span:nth-child(n + 3) {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .shead {
    color: var(--fg-2);
    font-weight: var(--fw-semibold);
    border-bottom: 1px solid var(--border);
    border-radius: 0;
  }
  .cname {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  .ctype {
    color: var(--fg-2);
  }
  .ctype.integer,
  .ctype.number {
    color: var(--syn-number);
  }
  .ctype.boolean {
    color: var(--syn-bool);
  }
  .ctype.date {
    color: var(--syn-string);
  }
  .dim {
    color: var(--fg-3);
  }
  .busy {
    width: 12px;
    height: 12px;
    border: 2px solid var(--bg-4);
    border-top-color: var(--accent);
    border-radius: 50%;
    opacity: 0;
    animation:
      spin 0.7s linear infinite,
      appear 1ms 250ms forwards;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @keyframes appear {
    to {
      opacity: 1;
    }
  }
</style>
