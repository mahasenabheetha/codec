<script lang="ts">
  import Braces from '@lucide/svelte/icons/braces'
  import Copy from '@lucide/svelte/icons/copy'
  import X from '@lucide/svelte/icons/x'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { isAbort } from '../../lib/api/client'
  import { logEntry, type LogEntry, type LogRow } from '../../lib/api/logs'
  import { copyText } from '../../lib/utils/clipboard'
  import FieldTree from './FieldTree.svelte'
  import { formatTime, formatWhen } from './format'
  import { maskSecrets } from './mask'
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
  const viewer = useViewer()

  let entry = $state.raw<LogEntry | null>(null)
  let around = $state.raw<LogRow[]>([])
  let error = $state<string | null>(null)
  let span = $state('5')

  $effect(() => {
    const i = index
    const n = Number(span)
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

<aside class="detail" aria-label="Entry">
  <header>
    <span class="title lv-{entry?.level || 'none'}">{entry?.level ? entry.level.toUpperCase() : 'Entry'}</span>
    {#if entry}<span class="where">{entry.file}:{entry.line}</span>{/if}
    <span class="spacer"></span>
    {#if entry}
      <IconButton icon={Copy} label="Copy the line as read (secrets masked)" size="sm" onclick={() => copy('line')} />
      <IconButton icon={Braces} label="Copy the entry as JSON (secrets masked)" size="sm" onclick={() => copy('entry')} />
    {/if}
    <IconButton icon={X} label="Close" shortcut="Escape" size="sm" onclick={onclose} />
  </header>
  <div class="body">
    {#if error}
      <p class="err">{error}</p>
    {:else if entry}
      {#if prettyMessage}
        <pre class="message json">{prettyMessage}</pre>
      {:else if entry.message}
        <p class="message">{entry.message}</p>
      {/if}
      {#if entry.error && !entry.message.includes(entry.error)}<p class="error">{entry.error}</p>{/if}

      <dl class="facts">
        {#if entry.time}
          <dt>Time</dt>
          <dd><span>{formatWhen(entry.time, prefs.time.value === 'utc' ? 'utc' : 'local')} <span class="muted">{entry.time}</span></span></dd>
        {/if}
        {#if entry.level}
          <dt>Level</dt>
          <dd><span>{entry.level}{#if entry.rawLevel && entry.rawLevel !== entry.level}<span class="muted"> ({entry.rawLevel})</span>{/if}</span><ValueActions key="level" value={entry.level} /></dd>
        {/if}
        {#each [['source', 'Source', entry.source], ['logger', 'Logger', entry.logger], ['caller', 'Caller', entry.caller], ['trace', 'Trace', entry.traceId]] as [key, label, v] (key)}
          {#if v}<dt>{label}</dt><dd><span>{v}</span><ValueActions key={key!} value={v} /></dd>{/if}
        {/each}
        <dt>From</dt>
        <dd><span>{entry.file}, line {entry.line}{entry.lines > 1 ? `–${entry.line + entry.lines - 1}` : ''} <span class="muted">· {entry.format}</span></span></dd>
      </dl>

      {#if entry.stack}
        <h3>Stack trace</h3>
        <StackTrace text={entry.stack} />
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
      <pre class="raw">{entry.raw}{#if entry.rawCut}<span class="muted"> … (cut at 1 MB)</span>{/if}</pre>
    {/if}
  </div>
</aside>

<style>
  .detail {
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
  .err {
    color: var(--err);
  }
</style>
