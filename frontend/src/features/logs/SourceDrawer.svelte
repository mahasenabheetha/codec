<script lang="ts" module>
  /** A place in the open folder the drawer can show: a reference's
   *  file, later a search hit (decision 99). */
  export interface SourceItem {
    file: string
    line: number
    col?: number
    endCol?: number // a search hit: its end on the line, exclusive
    quality?: 'exact' | 'strong' | 'possible'
    why?: string[] // what made it likely, for the badge's tip
    group?: string // a heading the list shows above it, e.g. a layer of an error
    /** A search hit's line as found, and the hits in it. */
    text?: string
    spans?: { start: number; end: number }[]
  }
</script>

<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ChevronUp from '@lucide/svelte/icons/chevron-up'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import SquareArrowOutUpRight from '@lucide/svelte/icons/square-arrow-out-up-right'
  import X from '@lucide/svelte/icons/x'
  import { getContent } from '../../lib/api/workspace'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { editorNav } from '../editor/active.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'

  // Where a log line points, without leaving the Log viewer: the files
  // that fit, and a read-only peek of the chosen one at the line. Only
  // "Open in Files" goes to the Files view.
  interface Props {
    title: string // what was looked for, e.g. "internal/x.go:12"
    items: SourceItem[] | null // null: no folder is open
    loading?: boolean
    error?: string | null
    note?: string // e.g. that the list stops at a limit
    empty?: string // said when nothing was found
    onclose: () => void
  }

  let { title, items, loading = false, error: failed = null, note, empty, onclose }: Props = $props()

  let pick = $state(0)
  $effect(() => {
    void items
    pick = 0
  })
  const item = $derived(items?.[Math.min(pick, items.length - 1)])

  // The peek: the file's lines, read once per file.
  const around = 400 // lines either side shown; the rest is a click away in Files
  let text = $state<string | null>(null)
  let error = $state<string | null>(null)
  let loaded = ''
  $effect(() => {
    const f = item?.file
    if (!f || f === loaded) return
    loaded = f
    text = error = null
    getContent(f)
      .then((c) => {
        if (loaded === f) text = c.text
      })
      .catch((e) => {
        if (loaded === f) error = e instanceof Error ? e.message : String(e)
      })
  })
  const lines = $derived.by(() => {
    if (text === null || !item) return []
    const all = text.split(/\r?\n/)
    const from = Math.max(0, item.line - 1 - around)
    return all.slice(from, item.line + around).map((t, i) => ({ n: from + i + 1, t }))
  })
  const past = $derived(text !== null && item ? item.line > text.split(/\r?\n/).length : false)

  let peek = $state<HTMLElement>()
  $effect(() => {
    void lines
    // Once the lines are in, the line itself in the middle.
    queueMicrotask(() => peek?.querySelector('.hit')?.scrollIntoView({ block: 'center' }))
  })

  function step(d: number) {
    if (items?.length) pick = (pick + d + items.length) % items.length
  }

  function openInFiles() {
    if (!item) return
    layout.openFile(item.file)
    editorNav.request(item.file, item.line, item.col ?? 1, item.endCol)
  }

  /** A line cut into plain and highlighted pieces. */
  function pieces(text: string, spans: { start: number; end: number }[]) {
    const out: { text: string; hit: boolean }[] = []
    let at = 0
    for (const sp of spans) {
      if (sp.start < at) continue
      if (sp.start > at) out.push({ text: text.slice(at, sp.start), hit: false })
      out.push({ text: text.slice(sp.start, sp.end), hit: true })
      at = sp.end
    }
    if (at < text.length) out.push({ text: text.slice(at), hit: false })
    return out
  }
  /** A search hit's line without its indentation, hits moved along. */
  function snippet(it: SourceItem) {
    const text = it.text ?? ''
    const lead = text.length - text.trimStart().length
    return pieces(text.slice(lead), (it.spans ?? []).map((sp) => ({ start: Math.max(0, sp.start - lead), end: sp.end - lead })))
  }
  // The hit on the peek's line: a search hit's columns.
  const hitSpan = $derived(item?.col && item.endCol ? [{ start: item.col - 1, end: item.endCol - 1 }] : [])

  const qualityTone = { exact: 'ok', strong: 'accent', possible: 'neutral' } as const
  const qualityTip = {
    exact: 'Very likely the place',
    strong: 'Likely the place',
    possible: 'Possibly the place',
  }
</script>

<section class="drawer" aria-label="Source">
  <header>
    <span class="title">Source</span>
    <span class="what" title={title}>{title}</span>
    <span class="spacer"></span>
    {#if items && items.length > 1}
      <span class="count">{pick + 1} of {items.length}</span>
      <IconButton icon={ChevronUp} label="Previous match" size="sm" onclick={() => step(-1)} />
      <IconButton icon={ChevronDown} label="Next match" size="sm" onclick={() => step(1)} />
    {/if}
    {#if item}
      <Button size="sm" variant="ghost" tone="open" icon={SquareArrowOutUpRight} onclick={openInFiles}>Open in Files</Button>
    {/if}
    <IconButton icon={X} label="Close the Source drawer" size="sm" onclick={onclose} />
  </header>

  {#if failed}
    <p class="empty err">{failed}</p>
  {:else if loading}
    <p class="empty muted">Searching the open folder…</p>
  {:else if items === null}
    <div class="empty">
      <p>Open the folder the log came from to see this file.</p>
      <Button size="sm" icon={FolderOpen} tone="open" onclick={() => ws.chooseFolder()}>Open folder…</Button>
    </div>
  {:else if items.length === 0}
    <p class="empty">{empty ?? 'No file of that name in the open folder'}{ws.info?.name ? ` (${ws.info.name})` : ''}.</p>
  {:else}
    {#if note}<p class="note">{note}</p>{/if}
    {#if items.length > 1 || items.some((it) => it.group)}
      <ul class="list">
        {#each items as it, i (i)}
          {#if it.group && it.group !== items[i - 1]?.group}<li class="group">{it.group}</li>{/if}
          <li>
            <button type="button" class:on={i === pick} onclick={() => (pick = i)}>
              <FileIcon file={ws.byPath.get(it.file) ?? { lang: 'text', path: it.file }} />
              <span class="file">{it.file}<span class="ln">:{it.line}</span></span>
              {#if it.text !== undefined}<span class="snippet">{#each snippet(it) as p, j (j)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>{/if}
              {#if it.quality}<Badge tone={qualityTone[it.quality]}>{it.quality}</Badge>{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
    {#if item}
      <div class="peek-head">
        <FileIcon file={ws.byPath.get(item.file) ?? { lang: 'text', path: item.file }} />
        <span class="file">{item.file}<span class="ln">:{item.line}</span></span>
        {#if item.quality}<span title={item.why?.length ? `${qualityTip[item.quality]}: ${item.why.join(', ')}` : qualityTip[item.quality]}><Badge tone={qualityTone[item.quality]}>{item.quality}</Badge></span>{/if}
      </div>
      <div class="peek" bind:this={peek}>
        {#if error}
          <p class="err">Can't read {item.file}: {error}</p>
        {:else if text === null}
          <p class="muted">Reading {item.file}…</p>
        {:else}
          {#if past}<p class="muted">The file has fewer lines than {item.line}: it may have changed since the log was written.</p>{/if}
          {#each lines as l (l.n)}
            <div class="row" class:hit={l.n === item.line}><span class="n">{l.n}</span><span class="t">{#if l.n === item.line && hitSpan.length}{#each pieces(l.t, hitSpan) as p, j (j)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}{:else}{l.t}{/if}</span></div>
          {/each}
        {/if}
      </div>
    {/if}
  {/if}
</section>

<style>
  .drawer {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    background: var(--bg-1);
    border-top: 1px solid var(--border);
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
  .what {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .spacer {
    flex: 1;
  }
  .count {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .empty {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-2);
    margin: 0;
    padding: var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .empty p {
    margin: 0;
  }
  .list {
    flex: none;
    max-height: 30%;
    margin: 0;
    padding: var(--s-1) 0;
    overflow: auto;
    list-style: none;
    border-bottom: 1px solid var(--border);
  }
  .list .group {
    padding: var(--s-1) var(--s-3) 2px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
  }
  .list button {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    padding: 2px var(--s-3);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
  }
  .list button:hover {
    background: var(--bg-2);
  }
  .list button.on {
    color: var(--fg-0);
    background: var(--accent-soft);
  }
  .file {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .ln {
    color: var(--fg-2);
  }
  .list .file {
    flex: 0 1 auto;
    max-width: 45%;
  }
  .snippet {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: pre;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  mark {
    color: inherit;
    background: color-mix(in oklab, var(--accent) 35%, transparent);
    border-radius: 2px;
  }
  .note {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
  .peek-head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-3);
    color: var(--fg-1);
  }
  .peek {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding-bottom: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: var(--lh-code);
    background: var(--bg-2);
  }
  .peek > p {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-family: var(--font-ui);
  }
  .row {
    display: flex;
    min-width: max-content;
  }
  .row.hit {
    background: var(--accent-soft);
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .n {
    flex: none;
    width: 5ch;
    padding-right: var(--s-2);
    text-align: right;
    color: var(--fg-2);
    user-select: none;
  }
  .t {
    white-space: pre;
    padding-right: var(--s-3);
  }
  .muted {
    color: var(--fg-2);
  }
  .err {
    color: var(--err);
  }
</style>
