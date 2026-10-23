<script lang="ts">
  import { tick } from 'svelte'
  import Columns3 from '@lucide/svelte/icons/columns-3'
  import Download from '@lucide/svelte/icons/download'
  import Logs from '@lucide/svelte/icons/logs'
  import Search from '@lucide/svelte/icons/search'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { exportLog, logGroups } from '../../lib/api/logs'
  import { shortcut } from '../../lib/stores/shortcuts.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { count, formatTime } from './format'
  import LogChart from './LogChart.svelte'
  import LogDetail from './LogDetail.svelte'
  import LogFields from './LogFields.svelte'
  import LogGroups from './LogGroups.svelte'
  import LogList from './LogList.svelte'
  import HighlightRules from './HighlightRules.svelte'
  import LogOverview from './LogOverview.svelte'
  import { maskSecrets } from './mask'
  import QueryInput from './QueryInput.svelte'
  import { logTabs, prefs, provideViewer, sourceName, type LogViewer, type RepeatMode, type TimeMode } from './viewer.svelte'

  // One open log, in a tab of the Log viewer: its files, overview,
  // search, chart and entries. Every tab keeps its own state, so going
  // back to one finds it as it was left.
  interface Props {
    viewer: LogViewer
    active: boolean // the tool is shown and this is its tab
  }

  let { viewer, active }: Props = $props()
  // The tab's components find this log through the context.
  // svelte-ignore state_referenced_locally
  provideViewer(viewer)

  let queryField = $state<HTMLInputElement>()
  let list = $state<LogList>()
  let tab = $state<'entries' | 'errors' | 'fields'>('entries')
  let columnFilter = $state('')

  // Search as you type, after a pause. A term still being written (a
  // field with no value yet, an open quote or bracket, a trailing OR)
  // waits for Enter, so it doesn't flash an error mid-word.
  const unfinished = (q: string) =>
    /[:<>=(\-]$/.test(q) || /\b(OR|AND|NOT)$/.test(q) || (q.match(/(?<!\\)"/g)?.length ?? 0) % 2 === 1 ||
    (q.match(/\(/g)?.length ?? 0) > (q.match(/\)/g)?.length ?? 0)
  $effect(() => {
    const q = viewer.input.trim()
    if (!viewer.opened || q === viewer.requested || unfinished(q)) return
    const t = setTimeout(() => {
      if (viewer.input.trim() === q && q !== viewer.requested) viewer.run()
    }, 350)
    return () => clearTimeout(t)
  })

  /** Select entry i and bring it into view in the entries. */
  async function jump(i: number) {
    tab = 'entries'
    const pos = await viewer.locate(i)
    if (pos < 0) return
    viewer.select(i, pos)
    await tick() // the list moves into the split view
    list?.reveal(pos)
  }

  /** Select the entry at a position in the matches (keyboard). */
  async function selectAt(pos: number, dir: 1 | -1 = 1) {
    if (!viewer.rowCount) return
    pos = Math.max(0, Math.min(viewer.rowCount - 1, pos))
    let r = await viewer.rowAsync(pos)
    // A gap holds no entry: step over it.
    while (r?.kind === 'gap' && pos + dir >= 0 && pos + dir < viewer.rowCount) {
      pos += dir
      r = await viewer.rowAsync(pos)
    }
    if (!r || r.kind === 'gap') return
    viewer.select(r.i, pos, r.lastI ?? r.i)
    await tick()
    list?.reveal(pos)
  }

  /** Open the selected fold, or fold back the lines it opened. */
  function toggleSelectedFold() {
    const r = viewer.selectedPos === null ? undefined : viewer.rowAt(viewer.selectedPos)
    if (r && (r.kind === 'output' || r.kind === 'repeat' || viewer.expanded.has(r.i))) viewer.toggleFold(r.i)
  }

  async function nextError(dir: 1 | -1) {
    const pos = await viewer.nextError(dir)
    if (pos === null) toast(dir > 0 ? 'No more errors below' : 'No errors above')
    else await selectAt(pos)
  }

  async function exportAs(format: 'text' | 'ndjson' | 'csv') {
    try {
      const blob = await viewer.withLog((id) => exportLog(id, { ...viewer.applied, format, columns: viewer.columns }))
      const name = (viewer.sources.length === 1 ? sourceName(viewer.sources[0]).replace(/\.[^.]+$/, '') : 'log') + '-filtered'
      const url = URL.createObjectURL(blob)
      Object.assign(document.createElement('a'), { href: url, download: `${name}.${format === 'text' ? 'log' : format}` }).click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }

  // The log in a few lines, for a ticket or a chat (secrets masked).
  async function copySummary() {
    const sm = viewer.opened?.summary
    if (!sm) return
    const { groups } = await viewer.withLog((id) => logGroups(id, { q: '' }))
    const lv = (k: string) => sm.levels[k] ?? 0
    const out = [
      `${viewer.sources.map(sourceName).join(', ')}: ${count(sm.entries)} entries${sm.from && sm.to ? `, ${sm.from} → ${sm.to}` : ''}`,
      `fatal ${lv('fatal')} · errors ${lv('error')} · warnings ${lv('warn')} · info ${lv('info')}`,
    ]
    if (viewer.applied.q) out.push(`Query: ${viewer.applied.q} → ${count(viewer.total)} entries`)
    if (groups.length) {
      out.push('', 'Most repeated:')
      for (const g of groups.slice(0, 5)) out.push(`  ${g.count}× [${g.level}] ${g.text}`)
    }
    const m = maskSecrets(out.join('\n'))
    copyText(m.text, 'Summary')
  }

  // j/k or the arrows move through the entries, e and Shift+E to the
  // next and previous error, / to the query.
  $effect(() => {
    const when = () => active && !!viewer.opened && tab === 'entries'
    const step = (d: number) => () =>
      selectAt(viewer.selectedPos === null ? (d > 0 ? 0 : viewer.rowCount - 1) : viewer.selectedPos + d, d > 0 ? 1 : -1)
    const offs = [
      shortcut('ArrowDown', step(1), { when }),
      shortcut('J', step(1), { when }),
      shortcut('ArrowUp', step(-1), { when }),
      shortcut('K', step(-1), { when }),
      shortcut('PageDown', step(20), { when }),
      shortcut('PageUp', step(-20), { when }),
      shortcut('Home', () => selectAt(0), { when }),
      shortcut('End', () => selectAt(viewer.rowCount - 1, -1), { when }),
      shortcut('E', () => nextError(1), { when }),
      shortcut('Shift+E', () => nextError(-1), { when }),
      shortcut('O', toggleSelectedFold, { when }),
      shortcut('/', () => queryField?.focus(), { when: () => active && !!viewer.opened }),
    ]
    return () => offs.forEach((off) => off())
  })

  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const days = $derived(!!s?.from && !!s?.to && new Date(s.to).getTime() - start > 86_400_000)
  const ranged = $derived(!!viewer.applied.from || !!viewer.applied.to)
  const fieldsShown = $derived((viewer.fieldKeys ?? []).filter((k) => k.toLowerCase().includes(columnFilter.toLowerCase())))

  toolShortcuts(() => active, {
    run: () => viewer.run(),
    copy: () => copySummary(),
    clear: () => {
      if (viewer.selected !== null) viewer.select(null)
      else if (viewer.input) viewer.input = ''
    },
  })
  toolStatus(() => active, () => {
    if (viewer.error) return { text: viewer.error, tone: 'err' }
    if (viewer.opening) return { text: 'Reading the log…' }
    if (!s) return { text: 'Ready' }
    const bad = (s.levels.error ?? 0) + (s.levels.fatal ?? 0)
    const text = `${count(s.entries)} entries · ${bad ? `${count(bad)} errors` : 'no errors'}`
    return { text: viewer.applied.q || ranged ? `${count(viewer.total)} of ${text}` : text, tone: bad ? 'err' : 'ok' }
  })
</script>

{#if !viewer.opened || !s}
  <div class="empty">
    {#if viewer.opening}
      <EmptyState icon={Logs} title="Reading the log…" description={viewer.sources.map(sourceName).join(', ')} />
    {:else}
      <EmptyState icon={Logs} title="Can't read this log" description={viewer.error ?? ''}>
        <Button onclick={() => logTabs.close(viewer)}>Close this tab</Button>
      </EmptyState>
    {/if}
  </div>
{:else}
  <div class="viewer">
    <div class="files">
      {#each viewer.sources as src, i (i)}
        {@const info = viewer.opened.files[i]}
        <span class="chip" title={info ? `${count(info.entries)} entries · ${count(info.lines)} lines${info.encoding ? ' · ' + info.encoding.join(', ') : ''}` : ''}>
          <span class="chip-name">{sourceName(src)}</span>
          {#if info}<span class="muted">{count(info.entries)}</span>{/if}
          <button type="button" class="chip-x" aria-label="Remove {sourceName(src)}" onclick={() => viewer.remove(i)}><X size={12} /></button>
        </span>
      {/each}
      <span class="spacer"></span>
      <Popover side="bottom" align="end">
        {#snippet trigger(props)}
          <Button {...props} size="sm" variant="ghost" icon={Download}>Export</Button>
        {/snippet}
        <div class="menu">
          <p class="muted">The {count(viewer.total)} matching entries as</p>
          <button type="button" onclick={() => exportAs('text')}>Lines as read <span class="muted">.log</span></button>
          <button type="button" onclick={() => exportAs('ndjson')}>One JSON object each <span class="muted">.ndjson</span></button>
          <button type="button" onclick={() => exportAs('csv')}>A table <span class="muted">.csv, with your columns</span></button>
        </div>
      </Popover>
    </div>

    <LogOverview summary={s} onjump={jump} />

    <div class="searchbar">
      <QueryInput
        bind:value={viewer.input}
        bind:field={queryField}
        invalid={!!viewer.queryError}
        fieldKeys={viewer.fieldKeys ?? []}
        fieldValues={viewer.fieldValues}
        sources={(s.sources ?? []).map((x) => x.value)}
        history={logTabs.history}
        onsubmit={() => viewer.run()}
        onclear={() => {
          viewer.input = ''
          viewer.run()
          queryField?.focus()
        }}
      />
      {#if ranged}
        <span class="chip range">
          {formatTime(viewer.applied.from, prefs.time.value, start, days) || 'start'} – {formatTime(viewer.applied.to, prefs.time.value, start, days) || 'end'}
          <button type="button" class="chip-x" aria-label="Clear the time range" onclick={() => viewer.setRange()}><X size={12} /></button>
        </span>
      {/if}
      <span class="total" aria-live="polite">
        {viewer.loading ? 'Searching…' : viewer.applied.q || ranged ? `${count(viewer.total)} of ${count(s.entries)} entries` : `${count(viewer.total)} entries`}{#if viewer.view.context && (viewer.applied.q || ranged)}<span class="muted">{` · with ${viewer.view.context} around each`}</span>{/if}
      </span>
    </div>
    <div class="optionbar">
      <span class="label">Level</span>
      <SegmentedControl
        label="Minimum level"
        options={[
          { value: '', label: 'All' },
          { value: 'info', label: 'Info+' },
          { value: 'warn', label: 'Warn+' },
          { value: 'error', label: 'Errors' },
        ]}
        value={viewer.minLevel}
        onchange={(v) => viewer.setLevel(v)}
      />
      <span class="label" title="Entries shown around each match, whatever the query (dimmed)">Around</span>
      <SegmentedControl
        label="Entries around each match"
        options={[
          { value: '0', label: 'Off' },
          { value: '3', label: '±3' },
          { value: '10', label: '±10' },
        ]}
        value={String(viewer.view.context)}
        onchange={(v) => viewer.setView({ context: Number(v) })}
      />
      <span class="label" title="Fold runs of plain output, and lines repeated one after another">Fold</span>
      <Toggle
        checked={viewer.view.output}
        label="Output"
        title="Fold runs of plain output lines (text without a level) into one row each"
        onchange={(on) => viewer.setView({ output: on })}
      />
      <SegmentedControl
        label="Fold repeated lines"
        options={[
          { value: 'off', label: 'No repeats' },
          { value: 'exact', label: 'Repeats' },
          { value: 'similar', label: 'Similar' },
        ]}
        value={viewer.view.repeats}
        onchange={(v: RepeatMode) => viewer.setView({ repeats: v })}
      />
      <span class="spacer"></span>
      <HighlightRules />
      <Popover side="bottom" align="end">
        {#snippet trigger(props)}
          <Button {...props} size="sm" variant="ghost" icon={Columns3}>Columns{viewer.columns.length ? ` (${viewer.columns.length})` : ''}</Button>
        {/snippet}
        <div class="columns">
          <input class="col-filter" bind:value={columnFilter} placeholder="Find a field" aria-label="Find a field" />
          {#if !viewer.fieldKeys}
            <p class="muted">Reading fields…</p>
          {:else if !viewer.fieldKeys.length}
            <p class="muted">This log has no fields, only text.</p>
          {/if}
          <div class="col-list">
            {#each fieldsShown as k (k)}
              <label><input type="checkbox" checked={prefs.columns.value.includes(k)} onchange={() => viewer.toggleColumn(k)} /> <span>{k}</span></label>
            {/each}
          </div>
        </div>
      </Popover>
      {#if s.timed > 0}
        <span class="label">Time</span>
        <SegmentedControl
          label="Times"
          options={[
            { value: 'local', label: 'Local' },
            { value: 'utc', label: 'UTC' },
            { value: 'relative', label: 'Since start' },
          ]}
          value={prefs.time.value}
          onchange={(v: TimeMode) => (prefs.time.value = v)}
        />
      {/if}
      <span class="label">Lines</span>
      <SegmentedControl
        label="Long lines"
        options={[
          { value: 'wrap', label: 'Wrap' },
          { value: 'scroll', label: 'One line' },
        ]}
        value={prefs.lines.value}
        onchange={(v: 'wrap' | 'scroll') => (prefs.lines.value = v)}
      />
    </div>
    {#if viewer.queryError}
      <p class="qerror">
        {viewer.queryError.message}
        {#if viewer.queryError.pos >= 0}<span class="muted">· at “{viewer.input.slice(viewer.queryError.pos, viewer.queryError.pos + 12)}”</span>{/if}
      </p>
    {:else if viewer.warnings.length}
      <p class="qwarn">{viewer.warnings.join(' · ')} — the term matches nothing</p>
    {/if}

    {#if s.timed === 0}
      <p class="notime">No times were found in this log, so there is no chart over time; the entries are in the order they were written. If its lines do start with a time, send the shape of one so codec learns it.</p>
    {:else if viewer.histogram?.buckets.length}
      <LogChart histogram={viewer.histogram} mode={prefs.time.value} {start} onrange={(a, b) => viewer.setRange(a, b)} />
    {/if}

    <Tabs
      bind:value={tab}
      label="Views"
      tabs={[
        { value: 'entries', label: 'Entries', count: viewer.total },
        { value: 'errors', label: 'Errors', count: s.groups },
        { value: 'fields', label: 'Fields' },
      ]}
    />
    <div class="main">
      {#if tab === 'errors'}
        <LogGroups onjump={jump} onshow={() => (tab = 'entries')} />
      {:else if tab === 'fields'}
        <LogFields onshow={() => (tab = 'entries')} />
      {:else if viewer.total === 0 && !viewer.loading}
        <EmptyState icon={Search} title="Nothing matches" description="Change the query or the time range." />
      {:else if viewer.selected !== null}
        <SplitPane id="logs" initial={0.62} min={260}>
          {#snippet first()}<LogList bind:this={list} onselect={(i, pos, last) => viewer.select(i, pos, last)} />{/snippet}
          {#snippet second()}<LogDetail index={viewer.selected!} onclose={() => viewer.select(null)} onpick={(i) => jump(i)} />{/snippet}
        </SplitPane>
      {:else}
        <LogList bind:this={list} onselect={(i, pos, last) => viewer.select(i, pos, last)} />
      {/if}
    </div>
  </div>
{/if}

<style>
  .empty {
    height: 100%;
    display: grid;
    place-items: center;
  }
  .viewer {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    min-height: 0;
  }
  .files,
  .searchbar,
  .optionbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .optionbar .label {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .optionbar .label:not(:first-child) {
    margin-left: var(--s-2);
  }
  .spacer {
    flex: 1;
  }
  .notime {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-1);
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-md);
  }
  .total {
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
    color: var(--fg-1);
  }
  .qerror,
  .qwarn {
    margin: 0;
    font-size: var(--fs-sm);
  }
  .qerror {
    color: var(--err);
  }
  .qwarn {
    color: var(--warn);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    max-width: 260px;
    height: var(--control-h-sm);
    padding: 0 2px 0 var(--s-2);
    font-size: var(--fs-sm);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .chip-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .chip.range {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--accent);
    border-color: var(--accent);
  }
  .chip-x {
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-full);
  }
  .chip-x:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
  .muted {
    color: var(--fg-2);
  }
  .main {
    flex: 1;
    min-height: 0;
  }
  .menu {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 260px;
    padding: var(--s-2);
  }
  .menu p {
    margin: 0 0 var(--s-1);
    font-size: var(--fs-xs);
  }
  .menu button {
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .menu button:hover {
    background: var(--bg-3);
  }
  .columns {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    width: 280px;
    padding: var(--s-2);
  }
  .col-filter {
    height: var(--control-h);
    padding: 0 var(--s-2);
  }
  .col-list {
    max-height: 320px;
    overflow: auto;
  }
  .col-list label {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: 2px 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .col-list span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .columns p {
    margin: 0;
    font-size: var(--fs-sm);
  }
</style>
