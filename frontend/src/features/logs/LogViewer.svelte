<script lang="ts">
  import { tick } from 'svelte'
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import Columns3 from '@lucide/svelte/icons/columns-3'
  import FileUp from '@lucide/svelte/icons/file-up'
  import Logs from '@lucide/svelte/icons/logs'
  import Search from '@lucide/svelte/icons/search'
  import TextWrap from '@lucide/svelte/icons/text-wrap'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import Dialog from '../../lib/components/Dialog.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { toolById } from '../../lib/tools'
  import { samples } from '../shared/samples'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { count, formatTime } from './format'
  import LogChart from './LogChart.svelte'
  import LogDetail from './LogDetail.svelte'
  import LogList from './LogList.svelte'
  import LogOverview from './LogOverview.svelte'
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

  async function jump(i: number) {
    const pos = await viewer.locate(i)
    if (pos < 0) return
    viewer.selected = i
    await tick() // the list moves into the split view
    list?.reveal(pos)
  }

  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const days = $derived(!!s?.from && !!s?.to && new Date(s.to).getTime() - start > 86_400_000)
  const ranged = $derived(!!viewer.applied.from || !!viewer.applied.to)
  const fieldsShown = $derived((viewer.fieldKeys ?? []).filter((k) => k.toLowerCase().includes(columnFilter.toLowerCase())))
  let columnFilter = $state('')

  toolShortcuts(() => active, {
    run: () => viewer.run(),
    copy: () => {},
    clear: () => {
      if (viewer.selected !== null) viewer.selected = null
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

          <form
            class="querybar"
            onsubmit={(e) => {
              e.preventDefault()
              viewer.run()
            }}
          >
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
            <div class="query" class:invalid={!!viewer.queryError}>
              <Search size={14} />
              <input
                bind:this={queryField}
                bind:value={viewer.input}
                list="log-history"
                aria-label="Query"
                placeholder={'Filter: words, "a phrase", level>=warn, service.name:api, -source:x, status>=500, /regex/'}
                spellcheck="false"
                autocomplete="off"
              />
              {#if viewer.input}
                <IconButton
                  icon={X}
                  label="Clear the query"
                  size="sm"
                  onclick={() => {
                    viewer.input = ''
                    viewer.run()
                    queryField?.focus()
                  }}
                />
              {/if}
            </div>
            <datalist id="log-history">
              {#each viewer.history as h (h)}<option value={h}></option>{/each}
            </datalist>
            {#if ranged}
              <span class="chip range">
                {formatTime(viewer.applied.from, prefs.time.value, start, days) || 'start'} – {formatTime(viewer.applied.to, prefs.time.value, start, days) || 'end'}
                <button type="button" class="chip-x" aria-label="Clear the time range" onclick={() => viewer.setRange()}><X size={12} /></button>
              </span>
            {/if}
            <span class="total" aria-live="polite">
              {viewer.loading ? '…' : viewer.applied.q || ranged ? `${count(viewer.total)} of ${count(s.entries)}` : `${count(viewer.total)}`}
            </span>
            <Popover side="bottom" align="end">
              {#snippet trigger(props)}
                <IconButton {...props} icon={Columns3} label="Columns" active={viewer.columns.length > 0} />
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
            <SegmentedControl
              label="Times"
              options={[
                { value: 'local', label: 'Local' },
                { value: 'utc', label: 'UTC' },
                { value: 'relative', label: '+time' },
              ]}
              value={prefs.time.value}
              onchange={(v: TimeMode) => (prefs.time.value = v)}
            />
            <IconButton icon={TextWrap} label="Wrap long messages" active={prefs.wrap.value} onclick={() => (prefs.wrap.value = !prefs.wrap.value)} />
          </form>
          {#if viewer.queryError}
            <p class="qerror">
              {viewer.queryError.message}
              {#if viewer.queryError.pos >= 0}<span class="muted">· at “{viewer.input.slice(viewer.queryError.pos, viewer.queryError.pos + 12)}”</span>{/if}
            </p>
          {:else if viewer.warnings.length}
            <p class="qwarn">{viewer.warnings.join(' · ')} — the term matches nothing</p>
          {/if}

          {#if viewer.histogram?.buckets.length}
            <LogChart histogram={viewer.histogram} mode={prefs.time.value} {start} onrange={(a, b) => viewer.setRange(a, b)} />
          {/if}

          <div class="main">
            {#if viewer.total === 0 && !viewer.loading}
              <EmptyState icon={Search} title="Nothing matches" description="Change the query or the time range." />
            {:else if viewer.selected !== null}
              <SplitPane id="logs" initial={0.62} min={260}>
                {#snippet first()}<LogList bind:this={list} onselect={(i) => (viewer.selected = i)} />{/snippet}
                {#snippet second()}<LogDetail index={viewer.selected!} onclose={() => (viewer.selected = null)} />{/snippet}
              </SplitPane>
            {:else}
              <LogList bind:this={list} onselect={(i) => (viewer.selected = i)} />
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
  .querybar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .query {
    flex: 1;
    min-width: 260px;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 2px 0 var(--s-2);
    color: var(--fg-2);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
  }
  .query:focus-within {
    border-color: var(--accent);
  }
  .query.invalid {
    border-color: var(--err);
  }
  .query input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: transparent;
    border: none;
    outline: none;
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
