<script lang="ts">
  import { tick } from 'svelte'
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import Columns3 from '@lucide/svelte/icons/columns-3'
  import FileUp from '@lucide/svelte/icons/file-up'
  import Logs from '@lucide/svelte/icons/logs'
  import Search from '@lucide/svelte/icons/search'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import Dialog from '../../lib/components/Dialog.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import { exportLog, logGroups } from '../../lib/api/logs'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { shortcut } from '../../lib/stores/shortcuts.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import Download from '@lucide/svelte/icons/download'
  import LogFields from './LogFields.svelte'
  import LogGroups from './LogGroups.svelte'
  import { maskSecrets } from './mask'
  import { samples } from '../shared/samples'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { count, formatTime } from './format'
  import LogChart from './LogChart.svelte'
  import LogDetail from './LogDetail.svelte'
  import LogList from './LogList.svelte'
  import LogOverview from './LogOverview.svelte'
  import QueryInput from './QueryInput.svelte'
  import { prefs, sourceName, viewer, type TimeMode } from './viewer.svelte'

  // Structured logs of any common format, read on the server and held
  // there in memory (decision 86): open, drop or paste one or more logs
  // (merged by time), then filter, chart and read them. Nothing is stored.
  let { active }: { active: boolean } = $props()

  const tool = toolById('logs')!
  let picker = $state<HTMLInputElement>()
  let queryField = $state<HTMLInputElement>()
  let list = $state<LogList>()
  let adding = false // the picker adds to the open log
  let pasteOpen = $state(false)
  let pasteText = $state('')

  // Text handed over by another tool (smart paste). Explorer → Open in
  // Log viewer calls viewer.open itself.
  $effect(() => {
    void handoff.pending
    const text = handoff.take('logs', 'viewer')
    if (text !== null) openPasted(text)
  })

  function pick(add: boolean) {
    adding = add
    picker?.click()
  }
  function openFiles(files: FileList | File[] | null | undefined, add = false) {
    const list = [...(files ?? [])]
    if (list.length) viewer.open(list.map((file) => ({ kind: 'file', file })), add && !!viewer.opened)
  }
  function openPasted(text: string, add = false) {
    if (!text.trim()) return
    const n = viewer.sources.filter((s) => s.kind === 'text').length
    viewer.open([{ kind: 'text', name: n && add ? `Pasted ${n + 1}` : 'Pasted', text }], add && !!viewer.opened)
  }

  // Files dropped anywhere on the tool; Ctrl+V outside a text box.
  let dropping = $state(false)
  const hasFile = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
  function ondragover(e: DragEvent) {
    if (!hasFile(e)) return
    e.preventDefault()
    e.stopPropagation() // see AnsibleTool: the desktop runtime listens at the root
    e.dataTransfer!.dropEffect = 'copy'
    dropping = true
  }
  function ondrop(e: DragEvent) {
    dropping = false
    if (!hasFile(e)) return
    e.preventDefault()
    e.stopPropagation()
    openFiles(e.dataTransfer?.files, e.shiftKey)
  }
  $effect(() => {
    if (!active) return
    const onpaste = (e: ClipboardEvent) => {
      const t = e.target as HTMLElement | null
      if (pasteOpen || t?.closest('input, textarea, [contenteditable="true"], .cm-editor')) return
      const text = e.clipboardData?.getData('text/plain') ?? ''
      if (text.trim()) {
        e.preventDefault()
        openPasted(text)
      }
    }
    window.addEventListener('paste', onpaste)
    return () => window.removeEventListener('paste', onpaste)
  })

  let tab = $state<'entries' | 'errors' | 'fields'>('entries')

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
  async function selectAt(pos: number) {
    if (!viewer.total) return
    pos = Math.max(0, Math.min(viewer.total - 1, pos))
    const r = await viewer.rowAsync(pos)
    if (!r) return
    viewer.select(r.i, pos)
    await tick()
    list?.reveal(pos)
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
    const step = (d: number) => () => selectAt(viewer.selectedPos === null ? (d > 0 ? 0 : viewer.total - 1) : viewer.selectedPos + d)
    const offs = [
      shortcut('ArrowDown', step(1), { when }),
      shortcut('J', step(1), { when }),
      shortcut('ArrowUp', step(-1), { when }),
      shortcut('K', step(-1), { when }),
      shortcut('PageDown', step(20), { when }),
      shortcut('PageUp', step(-20), { when }),
      shortcut('Home', () => selectAt(0), { when }),
      shortcut('End', () => selectAt(viewer.total - 1), { when }),
      shortcut('E', () => nextError(1), { when }),
      shortcut('Shift+E', () => nextError(-1), { when }),
      shortcut('/', () => queryField?.focus(), { when: () => active && !!viewer.opened }),
    ]
    return () => offs.forEach((off) => off())
  })

  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const days = $derived(!!s?.from && !!s?.to && new Date(s.to).getTime() - start > 86_400_000)
  const ranged = $derived(!!viewer.applied.from || !!viewer.applied.to)
  const fieldsShown = $derived((viewer.fieldKeys ?? []).filter((k) => k.toLowerCase().includes(columnFilter.toLowerCase())))
  let columnFilter = $state('')

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

<input
  bind:this={picker}
  type="file"
  multiple
  hidden
  accept=".log,.txt,.json,.ndjson,.jsonl,.clef,.out,.gz,text/plain,application/json,application/gzip"
  onchange={(e) => {
    const input = e.currentTarget as HTMLInputElement
    openFiles(input.files, adding)
    input.value = ''
  }}
/>

<div
  class="tool-wrap"
  class:dropping
  role="region"
  aria-label="Log viewer"
  ondragovercapture={ondragover}
  ondragleave={(e) => {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node | null)) dropping = false
  }}
  ondropcapture={ondrop}
>
  <ToolLayout {tool}>
    {#snippet controls()}
      {#each viewer.sources as src, i (i)}
        {@const info = viewer.opened?.files[i]}
        <span class="chip" title={info ? `${count(info.entries)} entries · ${count(info.lines)} lines${info.encoding ? ' · ' + info.encoding.join(', ') : ''}` : ''}>
          <span class="chip-name">{sourceName(src)}</span>
          {#if info}<span class="muted">{count(info.entries)}</span>{/if}
          <button type="button" class="chip-x" aria-label="Remove {sourceName(src)}" onclick={() => viewer.remove(i)}><X size={12} /></button>
        </span>
      {/each}
    {/snippet}
    {#snippet actions()}
      {#if viewer.opened}
        <Button icon={FileUp} onclick={() => pick(true)} title="Add logs; they are merged by time">Add logs…</Button>
        <Button icon={ClipboardPaste} onclick={() => (pasteOpen = true)}>Paste…</Button>
        <Popover side="bottom" align="end">
          {#snippet trigger(props)}
            <IconButton {...props} icon={Download} label="Export the matching entries" />
          {/snippet}
          <div class="menu">
            <p class="muted">The {count(viewer.total)} matching entries as</p>
            <button type="button" onclick={() => exportAs('text')}>Lines as read <span class="muted">.log</span></button>
            <button type="button" onclick={() => exportAs('ndjson')}>One JSON object each <span class="muted">.ndjson</span></button>
            <button type="button" onclick={() => exportAs('csv')}>A table <span class="muted">.csv, with your columns</span></button>
          </div>
        </Popover>
        <IconButton icon={X} label="Close the log" onclick={() => viewer.close()} />
      {:else}
        <Button icon={ClipboardPaste} onclick={() => (pasteOpen = true)}>Paste…</Button>
        <Button variant="primary" icon={FileUp} onclick={() => pick(false)}>Open logs…</Button>
      {/if}
    {/snippet}
    {#snippet body()}
      {#if !viewer.opened}
        <div class="empty">
          {#if viewer.opening}
            <EmptyState icon={Logs} title="Reading the log…" description={viewer.sources.map(sourceName).join(', ')} />
          {:else}
            <EmptyState
              icon={Logs}
              title={viewer.error ? "Can't read this log" : 'Read structured logs'}
              description={viewer.error ??
                'ECS and other JSON (slog, zap, logrus, zerolog…), logfmt, klog and plain text with stack traces, as argo logs, kubectl logs, containers or CI print them. Open or drop one or more files (merged by time), or paste with Ctrl+V. Nothing is stored.'}
            >
              <Button variant="primary" icon={FileUp} onclick={() => pick(false)}>Open logs…</Button>
              <Button icon={ClipboardPaste} onclick={() => (pasteOpen = true)}>Paste a log…</Button>
              <Button variant="ghost" onclick={() => openPasted(samples.logs)}>Try a sample</Button>
            </EmptyState>
          {/if}
        </div>
      {:else if s}
        <div class="viewer">
          <LogOverview summary={s} onjump={jump} />

          <div class="searchbar">
            <QueryInput
              bind:value={viewer.input}
              bind:field={queryField}
              invalid={!!viewer.queryError}
              fieldKeys={viewer.fieldKeys ?? []}
              fieldValues={viewer.fieldValues}
              sources={(s.sources ?? []).map((x) => x.value)}
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
              {viewer.loading ? 'Searching…' : viewer.applied.q || ranged ? `${count(viewer.total)} of ${count(s.entries)} entries` : `${count(viewer.total)} entries`}
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
            <span class="spacer"></span>
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
                {#snippet first()}<LogList bind:this={list} onselect={(i, pos) => viewer.select(i, pos)} />{/snippet}
                {#snippet second()}<LogDetail index={viewer.selected!} onclose={() => viewer.select(null)} onpick={(i) => jump(i)} />{/snippet}
              </SplitPane>
            {:else}
              <LogList bind:this={list} onselect={(i, pos) => viewer.select(i, pos)} />
            {/if}
          </div>
        </div>
      {/if}
    {/snippet}
  </ToolLayout>
  {#if dropping}<div class="drop" aria-hidden="true">Drop to read {viewer.opened ? '(hold Shift to add to this log)' : 'these logs'}</div>{/if}
</div>

<Dialog bind:open={pasteOpen} title="Paste a log" description="Any format; prefixes from argo logs, kubectl or CI are fine. It stays in memory only." width="720px">
  <textarea class="paste" bind:value={pasteText} spellcheck="false" placeholder="Paste here (Ctrl+V)"></textarea>
  <div class="paste-actions">
    {#if viewer.opened}
      <Button
        disabled={!pasteText.trim()}
        onclick={() => {
          openPasted(pasteText, true)
          pasteText = ''
          pasteOpen = false
        }}>Add to this log</Button
      >
    {/if}
    <Button
      variant="primary"
      disabled={!pasteText.trim()}
      onclick={() => {
        openPasted(pasteText)
        pasteText = ''
        pasteOpen = false
      }}>Read</Button
    >
  </div>
</Dialog>

<style>
  .tool-wrap {
    position: relative;
    height: 100%;
  }
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
  .optionbar .spacer {
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
  .paste {
    width: 100%;
    height: 320px;
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    resize: vertical;
  }
  .paste-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--s-2);
    margin-top: var(--s-3);
  }
  .drop {
    position: absolute;
    inset: var(--s-2);
    display: grid;
    place-items: center;
    font-size: var(--fs-lg);
    color: var(--accent);
    background: color-mix(in srgb, var(--bg-0) 85%, transparent);
    border: 2px dashed var(--accent);
    border-radius: var(--r-lg);
    pointer-events: none;
  }
</style>
