<script lang="ts">
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import { untrack } from 'svelte'
  import Braces from '@lucide/svelte/icons/braces'
  import PinIcon from '@lucide/svelte/icons/pin'
  import PinOff from '@lucide/svelte/icons/pin-off'
  import Copy from '@lucide/svelte/icons/copy'
  import FileSearch from '@lucide/svelte/icons/file-search'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import X from '@lucide/svelte/icons/x'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { isAbort } from '../../lib/api/client'
  import { findLine, findSource, logEntry, type LogEntry, type LogRow, type RefField, type SourceRef } from '../../lib/api/logs'
  import { copyText } from '../../lib/utils/clipboard'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import FieldTree from './FieldTree.svelte'
  import { formatTime, formatWhen } from './format'
  import Linked from './Linked.svelte'
  import { maskSecrets } from './mask'
  import SourceDrawer, { type SourceItem } from './SourceDrawer.svelte'
  import StackTrace from './StackTrace.svelte'
  import ValueActions from './ValueActions.svelte'
  import { prefs, useViewer } from './viewer.svelte'

  // One entry in full: message (JSON in it laid out), error, where it
  // came from, its stack trace, its fields as a tree, the entries just
  // before and after it whatever the query, and the line as read. Any
  // value can filter the log, become a column or be followed.
  interface Props {
    index: number
    onclose: () => void
    onpick: (i: number) => void // an entry around this one
  }

  let { index, onclose, onpick }: Props = $props()
  const maxFragment = 300 // logsrc.MaxFragment
  const viewer = useViewer()

  let entry = $state.raw<LogEntry | null>(null)
  let around = $state.raw<LogRow[]>([])
  let error = $state<string | null>(null)
  let span = $state('5')

  $effect(() => {
    const i = index
    const n = Number(span)
    void ws.info?.root // a folder opened from the drawer: find the files again
    const c = new AbortController()
    error = null
    viewer
      .withLog((id) => logEntry(id, i, n, c.signal))
      .then((r) => {
        entry = r.entry
        around = r.around
      })
      .catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    return () => c.abort()
  })

  // The Source drawer shows a file and line from the entry, or where a
  // piece of it selected appears in the folder; another entry closes it.
  type Drawer = { kind: 'ref'; ref: SourceRef } | { kind: 'search'; text: string } | { kind: 'line' }
  let drawer = $state<Drawer | null>(null)
  $effect(() => {
    void index
    drawer = null
  })
  const refItems = $derived.by((): SourceItem[] | null => {
    if (drawer?.kind !== 'ref' || !entry?.sources) return null
    const r = drawer.ref
    return (entry.sources[r.path] ?? []).map((m) => ({ file: m.file, line: r.line, col: r.col, quality: m.quality, why: [refWhy[m.quality]] }))
  })
  const refWhy = {
    exact: 'the path in the log names this file',
    strong: 'its folders agree with the log, but not all of them, or another file fits as well',
    possible: 'only the file name agrees with the log',
  }
  // Without a folder every reference may be found once one is open.
  const found = (r: SourceRef) => !entry?.sources || (entry.sources[r.path]?.length ?? 0) > 0
  const refs = (f: RefField) => entry?.refs?.[f] ?? []
  const show = (r: SourceRef) => (drawer = { kind: 'ref', ref: r })

  // A search runs when asked for, and again in a folder opened from
  // the drawer: of a selection, or of the entry's message (Find source).
  let hits = $state.raw<{ items: SourceItem[]; note?: string } | null>(null)
  let searching = $state(false)
  let searchError = $state<string | null>(null)
  $effect(() => {
    const d = drawer
    const open = !!ws.info?.open
    void ws.info?.root
    hits = null
    searchError = null
    searching = false
    // The entry is read, not followed: reloading it (±5 → ±20) needn't
    // search again; another entry closes the drawer anyway.
    const e = untrack(() => entry)
    if (!d || d.kind === 'ref' || !open || !e) return
    searching = true
    const c = new AbortController()
    if (d.kind === 'line') {
      findLine({ message: e.message, error: e.error, fields: e.fields.map((f) => f.key) }, c.signal)
        .then((r) => {
          const chain = r.layers.length > 1
          const items = r.layers.flatMap((l, i) =>
            l.hits.map((h) => ({ file: h.path, line: h.line, col: h.col, endCol: h.endCol, text: h.text, spans: h.spans, quality: h.quality, why: h.why, group: chain ? `${i + 1} · ${l.text}` : undefined })),
          )
          const missing = r.layers.filter((l) => !l.hits.length).map((l) => `“${l.text}”`)
          const notes = [
            !r.layers.length ? 'Nothing in the message is distinctive enough to search for; select a part of it instead' : '',
            chain ? `A wrapped error: ${r.layers.length} layers, each searched on its own` : '',
            missing.length && items.length ? `Not found: ${missing.join(', ')}` : '',
            r.timedOut ? 'The search stopped before the end of the folder' : r.truncated ? 'Very common words: only the first matches were scored' : '',
          ]
          hits = { items, note: notes.filter(Boolean).join('. ') || undefined }
          searching = false
        })
        .catch((err) => {
          if (isAbort(err)) return
          searchError = err instanceof Error ? err.message : String(err)
          searching = false
        })
      return () => c.abort()
    }
    findSource(d.text, c.signal)
      .then((r) => {
        const items = r.matches.map((m) => ({ file: m.path, line: m.line, col: m.col, endCol: m.endCol, text: m.text, spans: m.spans }))
        const files = new Set(items.map((i) => i.file)).size
        const count = `${items.length} ${items.length === 1 ? 'match' : 'matches'} in ${files} ${files === 1 ? 'file' : 'files'}`
        hits = { items, note: r.truncated ? `The first ${count}` : r.timedOut ? `${count}; the search stopped before the end of the folder` : count }
        searching = false
      })
      .catch((e) => {
        if (isAbort(e)) return
        searchError = e instanceof Error ? e.message : String(e)
        searching = false
      })
    return () => c.abort()
  })

  // Selecting text in the entry offers to search the folder for it.
  let body = $state<HTMLElement>()
  let aside = $state<HTMLElement>()
  let picked = $state<{ text: string; x: number; y: number } | null>(null)
  function checkSelection() {
    const sel = window.getSelection()
    const t = sel?.toString() ?? ''
    if (!sel || sel.rangeCount === 0 || !t.trim() || !body?.contains(sel.anchorNode) || !aside) {
      picked = null
      return
    }
    // The search is per line: of a selection over several, the longest.
    const line = t.split(/\r?\n/).reduce((a, l) => (l.trim().length > a.length ? l.trim() : a), '')
    const r = sel.getRangeAt(0).getBoundingClientRect()
    const box = aside.getBoundingClientRect()
    picked = { text: line, x: Math.min(Math.max(r.left - box.left, 8), box.width - 180), y: r.bottom - box.top + 4 }
  }
  $effect(() => {
    // A selection cleared (a click elsewhere) takes the button away.
    const off = () => {
      if (!window.getSelection()?.toString().trim()) picked = null
    }
    document.addEventListener('selectionchange', off)
    return () => document.removeEventListener('selectionchange', off)
  })
  function search() {
    if (!picked) return
    drawer = { kind: 'search', text: picked.text }
    picked = null
  }

  // A message that is JSON, laid out.
  const prettyMessage = $derived.by(() => {
    const m = entry?.message.trim() ?? ''
    if (!/^[[{]/.test(m)) return null
    try {
      return JSON.stringify(JSON.parse(m), null, 2)
    } catch {
      return null
    }
  })

  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const before = $derived(around.filter((r) => r.i < index))
  const after = $derived(around.filter((r) => r.i > index))

  /** The entry as a list row, for its pin. */
  function asRow(e: LogEntry): LogRow {
    return {
      i: e.i,
      file: Math.max(0, viewer.opened?.files.findIndex((f) => f.name === e.file) ?? 0),
      line: e.line,
      time: e.time,
      level: e.level,
      source: e.source,
      message: e.message,
      format: e.format,
      error: e.error,
      stack: !!e.stack,
    }
  }

  function copy(what: 'line' | 'entry') {
    if (!entry) return
    const text =
      what === 'line'
        ? entry.raw
        : JSON.stringify(
            {
              time: entry.time,
              level: entry.level || undefined,
              source: entry.source,
              message: entry.message,
              error: entry.error,
              stack: entry.stack,
              fields: Object.fromEntries(entry.fields.map((f) => [f.key, f.text])),
            },
            null,
            2,
          )
    const m = maskSecrets(text)
    copyText(m.text, `${what === 'line' ? 'Line' : 'Entry'}${m.masked ? ` (${m.masked} secret${m.masked === 1 ? '' : 's'} masked)` : ''}`)
  }
</script>

{#snippet rowLine(r: LogRow)}
  <button type="button" class="around lv-{r.level || 'none'}" onclick={() => onpick(r.i)}>
    <span class="t">{formatTime(r.time, prefs.time.value, start, false)}</span>
    <span class="lv">{r.level ?? ''}</span>
    <span class="m">{r.message || r.error}</span>
  </button>
{/snippet}

<aside class="detail" aria-label="Entry" bind:this={aside}>
  <header>
    <span class="title lv-{entry?.level || 'none'}">{entry?.level ? entry.level.toUpperCase() : 'Entry'}</span>
    {#if entry}<span class="where">{entry.file}:{entry.line}</span>{/if}
    <span class="spacer"></span>
    {#if entry && (entry.message || entry.error)}
      <IconButton
        icon={FileSearch}
        label="Find source: where this line was written in the open folder"
        size="sm"
        active={drawer?.kind === 'line'}
        onclick={() => (drawer = drawer?.kind === 'line' ? null : { kind: 'line' })}
      />
    {/if}
    {#if entry}
      <IconButton icon={Copy} tone="copy" label="Copy the line as read (secrets masked)" size="sm" onclick={() => copy('line')} />
      <IconButton icon={Braces} label="Copy the entry as JSON (secrets masked)" size="sm" onclick={() => copy('entry')} />
      <IconButton
        icon={viewer.isPinned(entry.i) ? PinOff : PinIcon}
        label={viewer.isPinned(entry.i) ? 'Unpin' : 'Pin this entry'}
        shortcut="P"
        size="sm"
        active={viewer.isPinned(entry.i)}
        onclick={() => entry && viewer.togglePin(entry.i, asRow(entry))}
      />
    {/if}
    <IconButton icon={X} label="Close" shortcut="Escape" size="sm" onclick={onclose} />
  </header>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="body" bind:this={body} onmouseup={checkSelection} onkeyup={checkSelection} onscroll={() => (picked = null)}>
    {#if error}
      <ErrorNote message={error} />
    {:else if entry}
      {#if prettyMessage}
        <pre class="message json">{prettyMessage}</pre>
      {:else if entry.message}
        <p class="message"><Linked text={entry.message} refs={refs('message')} {found} onref={show} /></p>
      {/if}
      {#if entry.error && !entry.message.includes(entry.error)}<p class="error"><Linked text={entry.error} refs={refs('error')} {found} onref={show} /></p>{/if}

      <dl class="facts">
        {#if entry.time}
          <dt>Time</dt>
          <dd><span>{formatWhen(entry.time, prefs.time.value === 'utc' ? 'utc' : 'local')} <span class="muted">{entry.time}</span></span></dd>
        {/if}
        {#if entry.level}
          <dt>Level</dt>
          <dd><span>{entry.level}{#if entry.rawLevel && entry.rawLevel !== entry.level}<span class="muted"> ({entry.rawLevel})</span>{/if}</span><ValueActions key="level" value={entry.level} /></dd>
        {/if}
        {#each [['step', 'Step', entry.step], ['source', 'Source', entry.source], ['logger', 'Logger', entry.logger], ['caller', 'Caller', entry.caller], ['trace', 'Trace', entry.traceId]] as [key, label, v] (key)}
          {#if v}<dt>{label}</dt><dd><span>{#if key === 'caller'}<Linked text={v} refs={refs('caller')} {found} onref={show} />{:else}{v}{/if}</span><ValueActions key={key!} value={v} /></dd>{/if}
        {/each}
        <dt>From</dt>
        <dd><span>{entry.file}, line {entry.line}{entry.lines > 1 ? `–${entry.line + entry.lines - 1}` : ''} <span class="muted">· {entry.format}</span></span></dd>
      </dl>

      {#if entry.stack}
        <h3>Stack trace</h3>
        <StackTrace text={entry.stack} refs={refs('stack')} {found} onref={show} />
      {/if}

      {#if entry.fields.length}
        <h3>Fields <span class="muted">{entry.fields.length}</span></h3>
        <FieldTree fields={entry.fields} />
      {/if}

      <div class="around-head">
        <h3>Around it <span class="muted">whatever the query</span></h3>
        <SegmentedControl
          label="Entries around"
          options={[
            { value: '5', label: '±5' },
            { value: '20', label: '±20' },
            { value: '50', label: '±50' },
          ]}
          bind:value={span}
        />
      </div>
      <div class="around-list">
        {#each before as r (r.i)}{@render rowLine(r)}{/each}
        <div class="around current lv-{entry.level || 'none'}">
          <span class="t">{formatTime(entry.time, prefs.time.value, start, false)}</span>
          <span class="lv">{entry.level ?? ''}</span>
          <span class="m">{entry.message || entry.error}</span>
        </div>
        {#each after as r (r.i)}{@render rowLine(r)}{/each}
      </div>

      <h3>As read</h3>
      <pre class="raw"><Linked text={entry.raw} refs={refs('raw')} {found} onref={show} />{#if entry.rawCut}<span class="muted"> … (cut at 1 MB)</span>{/if}</pre>
    {/if}
  </div>
  {#if picked}
    <button
      type="button"
      class="find"
      style:left="{picked.x}px"
      style:top="{picked.y}px"
      disabled={picked.text.length > maxFragment}
      title={picked.text.length > maxFragment ? 'Select a shorter piece of the line to search for' : `Search the open folder for “${picked.text}”`}
      onmousedown={(e) => e.preventDefault()}
      onclick={search}><TextSearch size={14} strokeWidth={1.75} />Find in workspace</button
    >
  {/if}
  {#if drawer}
    <div class="source">
      {#if drawer.kind === 'ref'}
        <SourceDrawer title="{drawer.ref.path}:{drawer.ref.line}" items={refItems} onclose={() => (drawer = null)} />
      {:else if drawer.kind === 'line'}
        <SourceDrawer
          title="Where this line was written"
          items={ws.info?.open ? (hits?.items ?? []) : null}
          loading={searching}
          error={searchError}
          note={hits?.note}
          empty="Not found in the open folder"
          onclose={() => (drawer = null)}
        />
      {:else}
        <SourceDrawer
          title="“{drawer.text}”"
          items={ws.info?.open ? (hits?.items ?? []) : null}
          loading={searching}
          error={searchError}
          note={hits?.note}
          empty="Not found in the open folder"
          onclose={() => (drawer = null)}
        />
      {/if}
    </div>
  {/if}
</aside>

<style>
  .detail {
    position: relative;
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2) var(--s-1) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .title {
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--fg-1);
  }
  .where {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .lv-error,
  .lv-fatal {
    --lv: var(--err);
  }
  .lv-warn {
    --lv: var(--warn);
  }
  .lv-info {
    --lv: var(--info);
  }
  .title.lv-error,
  .title.lv-fatal,
  .title.lv-warn,
  .title.lv-info {
    color: var(--lv);
  }
  .spacer {
    flex: 1;
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
  }
  .message {
    margin: 0 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .error {
    margin: 0 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--err);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .facts {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 2px var(--s-3);
    margin: 0 0 var(--s-2);
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--fg-2);
  }
  dd {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    margin: 0;
    overflow-wrap: anywhere;
  }
  dd > span {
    min-width: 0;
  }
  h3 {
    margin: var(--s-4) 0 var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  .muted {
    color: var(--fg-2);
    font-weight: var(--fw-regular);
    text-transform: none;
    letter-spacing: 0;
  }
  pre {
    margin: 0;
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: var(--lh-code);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--bg-2);
    border-radius: var(--r-sm);
  }
  .around-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: var(--s-2);
  }
  .around-list {
    margin-top: var(--s-1);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .around {
    display: grid;
    grid-template-columns: 96px 44px minmax(0, 1fr);
    gap: var(--s-2);
    width: 100%;
    padding: 2px var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
  }
  button.around:hover {
    background: var(--bg-2);
    color: var(--fg-0);
  }
  .around.current {
    color: var(--fg-0);
    background: var(--accent-soft);
  }
  .around .lv {
    text-transform: uppercase;
    color: var(--lv, var(--fg-2));
  }
  .around .m,
  .around .t {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .find {
    position: absolute;
    z-index: var(--z-popover);
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    height: 26px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-pop);
  }
  .find:hover:not(:disabled) {
    color: var(--accent);
  }
  .find:disabled {
    color: var(--fg-2);
  }
  .source {
    flex: none;
    height: 55%;
    min-height: 160px;
    overflow: hidden;
    border-radius: 0 0 var(--r-md) var(--r-md);
  }
</style>
