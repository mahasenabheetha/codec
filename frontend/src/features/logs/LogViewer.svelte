<script lang="ts">
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import FilePlus from '@lucide/svelte/icons/file-plus'
  import FileUp from '@lucide/svelte/icons/file-up'
  import Logs from '@lucide/svelte/icons/logs'
  import Plus from '@lucide/svelte/icons/plus'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import Dialog from '../../lib/components/Dialog.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { toolById } from '../../lib/tools'
  import { samples } from '../shared/samples'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import { toolStatus } from '../shared/tool.svelte'
  import { count } from './format'
  import LogTab from './LogTab.svelte'
  import { logTabs, sourceName, type LogViewer } from './viewer.svelte'

  // Structured logs of any common format, read on the server and held
  // there in memory (decision 86). Each log opened is a tab of its own
  // (one or more files merged by time); "Add" merges into the current
  // tab instead. Nothing is stored.
  let { active }: { active: boolean } = $props()

  const tool = toolById('logs')!
  let picker = $state<HTMLInputElement>()
  let adding = false // the picker adds to the current tab
  let pasteOpen = $state(false)
  let pasteText = $state('')

  // Text handed over by another tool (smart paste). Explorer → Open in
  // Log viewer opens its tab itself.
  $effect(() => {
    void handoff.pending
    const text = handoff.take('logs', 'viewer')
    if (text !== null) openPasted(text)
  })

  const cur = $derived(logTabs.current)

  function pick(add: boolean) {
    adding = add
    picker?.click()
  }
  function openFiles(files: FileList | File[] | null | undefined, add = false) {
    const list = [...(files ?? [])]
    if (list.length) logTabs.open(list.map((file) => ({ kind: 'file', file })), add)
  }
  function openPasted(text: string, add = false, name?: string) {
    if (!text.trim()) return
    // Pasted logs are numbered across the tabs, so tabs and chips tell them apart.
    const n = logTabs.list.reduce((k, v) => k + v.sources.filter((s) => s.kind === 'text' && s.name.startsWith('Pasted')).length, 0)
    logTabs.open([{ kind: 'text', name: name ?? (n ? `Pasted ${n + 1}` : 'Pasted'), text }], add)
  }

  // Files dropped anywhere on the tool (Shift adds them to the current
  // tab); Ctrl+V outside a text box opens what was copied in a tab.
  let dropping = $state(false)
  let shift = $state(false)
  const hasFile = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
  function ondragover(e: DragEvent) {
    if (!hasFile(e)) return
    e.preventDefault()
    e.stopPropagation() // see AnsibleTool: the desktop runtime listens at the root
    e.dataTransfer!.dropEffect = 'copy'
    dropping = true
    shift = e.shiftKey
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

  const mb = (n: number) => (n < 1 << 20 ? `${Math.max(1, Math.round(n / 1024))} KB` : `${(n / (1 << 20)).toFixed(n < 10 << 20 ? 1 : 0)} MB`)
  function tip(v: LogViewer): string {
    const o = v.opened
    const names = v.sources.map(sourceName).join('\n')
    return o ? `${names}\n${count(o.summary.entries)} entries · ${mb(o.size)} held in memory` : names
  }

  toolStatus(() => active && !logTabs.list.length, () => ({ text: 'Ready' }))
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
    {#snippet actions()}
      {#if cur?.opened}
        <Button icon={FilePlus} onclick={() => pick(true)} title="Add logs to this tab; they are merged by time">Add to this tab…</Button>
      {/if}
      <Button icon={ClipboardPaste} onclick={() => (pasteOpen = true)}>Paste…</Button>
      <Button variant="primary" icon={FileUp} tone="open" onclick={() => pick(false)} title="Each opening is a tab of its own; several files at once are merged by time">Open logs…</Button>
    {/snippet}
    {#snippet body()}
      {#if !logTabs.list.length}
        <div class="empty">
          <EmptyState
            icon={Logs}
            title="Read structured logs"
            description="ECS and other JSON (slog, zap, logrus, zerolog…), logfmt, klog and plain text with stack traces, as argo logs, kubectl logs, containers or CI print them. Open or drop one or more files (merged by time), or paste with Ctrl+V; each log you open gets a tab. Nothing is stored."
          >
            <Button variant="primary" icon={FileUp} tone="open" onclick={() => pick(false)}>Open logs…</Button>
            <Button icon={ClipboardPaste} onclick={() => (pasteOpen = true)}>Paste a log…</Button>
            <Button variant="ghost" onclick={() => openPasted(samples.logs, false, 'Sample')}>Try a sample</Button>
          </EmptyState>
        </div>
      {:else}
        <div class="tabs">
          <div class="strip" role="tablist" aria-label="Open logs">
            {#each logTabs.list as v (v.key)}
              <div class="tab" class:on={v === cur} class:bad={!!v.error && !v.opened}>
                <button type="button" role="tab" aria-selected={v === cur} class="name" title={tip(v)} onclick={() => logTabs.select(v)}>
                  <span class="label">{v.opening && !v.opened ? 'Reading…' : v.title}</span>
                  {#if v.opened}<span class="n">{count(v.opened.summary.entries)}</span>{/if}
                </button>
                <button type="button" class="close" aria-label="Close {v.title}" title="Close this log" onclick={() => logTabs.close(v)}><X size={12} /></button>
              </div>
            {/each}
            <IconButton icon={Plus} tone="add" label="Open logs in a new tab" size="sm" onclick={() => pick(false)} />
          </div>
          {#each logTabs.list as v (v.key)}
            <div class="tab-body" class:hidden={v !== cur}>
              <LogTab viewer={v} active={active && v === cur} />
            </div>
          {/each}
        </div>
      {/if}
    {/snippet}
  </ToolLayout>
  {#if dropping}
    <div class="drop" aria-hidden="true">
      {cur?.opened && shift ? 'Drop to add to this tab' : 'Drop to open in a new tab (hold Shift to add to this one)'}
    </div>
  {/if}
</div>

<Dialog bind:open={pasteOpen} title="Paste a log" description="Any format; prefixes from argo logs, kubectl or CI are fine. It stays in memory only." width="720px">
  <textarea class="paste" bind:value={pasteText} spellcheck="false" placeholder="Paste here (Ctrl+V)"></textarea>
  <div class="paste-actions">
    {#if cur?.opened}
      <Button
        disabled={!pasteText.trim()}
        onclick={() => {
          openPasted(pasteText, true)
          pasteText = ''
          pasteOpen = false
        }}>Add to this tab</Button
      >
    {/if}
    <Button
      variant="primary"
      disabled={!pasteText.trim()}
      onclick={() => {
        openPasted(pasteText)
        pasteText = ''
        pasteOpen = false
      }}>Open in a new tab</Button
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
  .tabs {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    min-height: 0;
  }
  .strip {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    overflow-x: auto;
    border-bottom: 1px solid var(--border);
  }
  .tab {
    display: flex;
    align-items: center;
    flex: none;
    max-width: 240px;
    margin-bottom: -1px;
    color: var(--fg-1);
    border: 1px solid transparent;
    border-bottom: none;
    border-radius: var(--r-md) var(--r-md) 0 0;
  }
  .tab:hover {
    background: var(--bg-2);
  }
  .tab.on {
    color: var(--fg-0);
    background: var(--bg-1);
    border-color: var(--border);
  }
  .tab.bad .label {
    color: var(--err);
  }
  .name {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-1) var(--s-1) var(--s-1) var(--s-3);
    font-size: var(--fs-sm);
    color: inherit;
    background: none;
    border: none;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .n {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    font-variant-numeric: tabular-nums;
  }
  .close {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    margin-right: var(--s-1);
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .close:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
  .tab-body {
    flex: 1;
    min-height: 0;
  }
  .tab-body.hidden {
    display: none;
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
