<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import X from '@lucide/svelte/icons/x'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { isAbort } from '../../lib/api/client'
  import { logEntry, type LogEntry } from '../../lib/api/logs'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatWhen } from './format'
  import { prefs, viewer } from './viewer.svelte'

  // One entry in full: its core fields, message, error, stack, every
  // field and the line(s) it was read from.
  interface Props {
    index: number
    onclose: () => void
  }

  let { index, onclose }: Props = $props()

  let entry = $state.raw<LogEntry | null>(null)
  let error = $state<string | null>(null)

  $effect(() => {
    const i = index
    const c = new AbortController()
    error = null
    viewer
      .withLog((id) => logEntry(id, i, 0, c.signal))
      .then((r) => (entry = r.entry))
      .catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    return () => c.abort()
  })

  const facts = $derived(
    entry
      ? ([
          ['Time', entry.time ? `${formatWhen(entry.time, prefs.time.value === 'utc' ? 'utc' : 'local')}  ·  ${entry.time}` : ''],
          ['Level', entry.level ? `${entry.level}${entry.rawLevel && entry.rawLevel !== entry.level ? ` (${entry.rawLevel})` : ''}` : ''],
          ['Source', entry.source ?? ''],
          ['Logger', entry.logger ?? ''],
          ['Caller', entry.caller ?? ''],
          ['Trace', entry.traceId ?? ''],
          ['From', `${entry.file}, line ${entry.line}${entry.lines > 1 ? `–${entry.line + entry.lines - 1}` : ''} · ${entry.format}`],
        ].filter(([, v]) => v) as [string, string][])
      : [],
  )
</script>

<aside class="detail" aria-label="Entry">
  <header>
    <span class="title lv-{entry?.level || 'none'}">{entry?.level ? entry.level.toUpperCase() : 'Entry'}</span>
    <span class="spacer"></span>
    {#if entry}<IconButton icon={Copy} label="Copy the line as read" size="sm" onclick={() => copyText(entry!.raw, 'Line')} />{/if}
    <IconButton icon={X} label="Close" size="sm" onclick={onclose} />
  </header>
  <div class="body">
    {#if error}
      <p class="err">{error}</p>
    {:else if entry}
      {#if entry.message}<p class="message">{entry.message}</p>{/if}
      {#if entry.error && !entry.message.includes(entry.error)}<p class="error">{entry.error}</p>{/if}
      <dl>
        {#each facts as [k, v] (k)}<dt>{k}</dt><dd>{v}</dd>{/each}
      </dl>
      {#if entry.stack}
        <h3>Stack trace</h3>
        <pre class="stack">{entry.stack}</pre>
      {/if}
      {#if entry.fields.length}
        <h3>Fields <span class="muted">{entry.fields.length}</span></h3>
        <dl class="fields">
          {#each entry.fields as f, i (i)}<dt title={f.key}>{f.key}</dt><dd class="k-{f.kind}">{f.text}</dd>{/each}
        </dl>
      {/if}
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
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2) var(--s-1) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .title {
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--fg-1);
  }
  .lv-error,
  .lv-fatal {
    color: var(--err);
  }
  .lv-warn {
    color: var(--warn);
  }
  .lv-info {
    color: var(--info);
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
  dl {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 2px var(--s-3);
    margin: 0 0 var(--s-3);
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--fg-2);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .fields dt {
    font-family: var(--font-mono);
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .fields dd {
    font-family: var(--font-mono);
    white-space: pre-wrap;
  }
  .k-number {
    color: var(--syn-number);
  }
  .k-bool,
  .k-null {
    color: var(--syn-bool);
  }
  h3 {
    margin: var(--s-3) 0 var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
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
  .muted {
    color: var(--fg-2);
    font-weight: var(--fw-regular);
  }
  .err {
    color: var(--err);
  }
</style>
