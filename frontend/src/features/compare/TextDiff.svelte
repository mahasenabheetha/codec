<script lang="ts">
  import { tick } from 'svelte'
  import ArrowDown from '@lucide/svelte/icons/arrow-down'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down'
  import Copy from '@lucide/svelte/icons/copy'
  import CodeView from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import type { CompareResult, DiffCell, DiffRow } from '../../lib/api/compare'
  import { copyText } from '../../lib/utils/clipboard'

  // A text diff side by side: each changed line beside the line that
  // replaced it, the changed characters marked strongly, unchanged runs
  // folded into gaps that open on click. Unified shows the patch.
  // opts holds the layout and the ignore flags: the Compare tab's, or a
  // tool's own (Text & data → Text diff).
  interface Props {
    result: CompareResult
    opts: { textView: 'split' | 'unified'; ignoreSpace: boolean; ignoreCase: boolean }
  }

  let { result, opts }: Props = $props()

  const rows = $derived(result.rows ?? [])
  // How much of each gap is open, by row index: lines shown at each of
  // its ends (a click opens STEP more; "all" opens the rest). CompareView
  // keys this component on the result, so a new result starts folded.
  const STEP = 20
  let opened = $state(new Map<number, number>())
  let list = $state<HTMLElement>()
  // The rail of marks is a map of a diff longer than its box: one that
  // fits needs none (its marks would not line up with the rows).
  let scrolls = $state(false)
  $effect(() => {
    if (!list) return
    const el = list
    const measure = () => (scrolls = el.scrollHeight > el.clientHeight + 1)
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    void [rows, opened]
    queueMicrotask(measure)
    return () => ro.disconnect()
  })

  // Changes as blocks (a run of changed rows), for next and previous.
  const blocks = $derived(rows.flatMap((r, i) => (r.kind !== 'equal' && r.kind !== 'gap' && (i === 0 || rows[i - 1].kind === 'equal' || rows[i - 1].kind === 'gap') ? [i] : [])))
  let at = $state(-1)
  function go(step: number) {
    if (!blocks.length) return
    at = at < 0 ? (step > 0 ? 0 : blocks.length - 1) : (at + step + blocks.length) % blocks.length
    const el = list?.querySelector<HTMLElement>(`[data-block="${at}"]`)
    el?.scrollIntoView({ block: 'center' })
    el?.focus({ preventScroll: true })
  }
  function onkeydown(e: KeyboardEvent) {
    if (!e.altKey || (e.key !== 'ArrowDown' && e.key !== 'ArrowUp')) return
    e.preventDefault()
    go(e.key === 'ArrowDown' ? 1 : -1)
  }

  async function unfoldAt(i: number, all = false) {
    opened = new Map(opened).set(i, all ? Infinity : (opened.get(i) ?? 0) + STEP)
    await tick()
    list?.querySelector<HTMLElement>(`[data-from="${i}"]`)?.focus()
  }

  const lines = $derived({ left: result.left.text.split(/\r?\n/), right: result.right.text.split(/\r?\n/) })

  /** A gap's unchanged lines from..to (of its count), as equal rows. */
  function unfold(r: DiffRow, from = 0, to = r.count ?? 0): DiffRow[] {
    const out: DiffRow[] = []
    for (let k = from; k < to; k++) {
      const l = r.left!.line + k
      const rr = r.right!.line + k
      out.push({ kind: 'equal', left: { line: l, text: lines.left[l - 1] ?? '' }, right: { line: rr, text: lines.right[rr - 1] ?? '' } })
    }
    return out
  }

  const parts = $derived.by(() => {
    const n = { change: 0, delete: 0, insert: 0 }
    for (const r of rows) if (r.kind !== 'equal' && r.kind !== 'gap') n[r.kind]++
    return [
      { n: n.change, what: 'changed', cls: 'chg' },
      { n: n.delete, what: 'removed', cls: 'del' },
      { n: n.insert, what: 'added', cls: 'add' },
    ].filter((p) => p.n > 0)
  })
  // Where the first difference starts, for near-identical strings.
  const multiline = $derived([result.left.text, result.right.text].some((t) => /\n./.test(t)))
  const first = $derived.by(() => {
    const r = rows.find((x) => x.kind !== 'equal' && x.kind !== 'gap')
    if (!r) return ''
    const line = `line ${(r.left ?? r.right)!.line}`
    const starts = [r.left?.spans?.[0]?.[0], r.right?.spans?.[0]?.[0]].filter((n) => n !== undefined)
    if (r.kind !== 'change' || !starts.length) return multiline ? line : ''
    const at = `character ${Math.min(...starts) + 1}`
    return multiline ? `${line}, ${at}` : at
  })

  // Wide enough for the biggest line number on either side.
  const lnWidth = $derived(`${String(Math.max(lines.left.length, lines.right.length)).length + 2}ch`)

  /** A cell's text split into plain and changed pieces. */
  function pieces(c: DiffCell) {
    const out: { text: string; hit: boolean }[] = []
    let at = 0
    for (const [s, e] of c.spans ?? []) {
      if (s > at) out.push({ text: c.text.slice(at, s), hit: false })
      out.push({ text: c.text.slice(s, e), hit: true })
      at = e
    }
    if (at < c.text.length || out.length === 0) out.push({ text: c.text.slice(at), hit: false })
    return out
  }

  const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}`

  // The patch, line by line, for the coloured unified view.
  const patch = $derived((result.diff ?? '').split(/\r?\n/).map((text) => ({ text, kind: text.startsWith('@@') ? 'hunk' : text.startsWith('+++') || text.startsWith('---') ? 'file' : text.startsWith('+') ? 'add' : text.startsWith('-') ? 'del' : '' })))
  // Where the changes are, down the right edge: a mark per block of
  // changed rows, as far down as the block is in the diff.
  const marks = $derived(
    blocks.map((i, b) => ({ b, top: rows.length ? (i / rows.length) * 100 : 0, kind: rows[i].kind })),
  )
</script>

{#snippet cell(c: DiffCell | undefined, side: 'left' | 'right', kind: DiffRow['kind'])}
  {#if c}
    <span class="ln {side}">{#if kind !== 'equal'}<span class="sr-only">{side === 'left' ? 'old' : 'new'} </span>{/if}{c.line}</span>
    <span class="sign {side}" class:tint={kind !== 'equal'} aria-hidden="true">{kind === 'equal' ? '' : side === 'left' ? '−' : '+'}</span>
    <span class="text {side}" class:tint={kind !== 'equal'}>{#each pieces(c) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>
  {:else}
    <span class="ln {side}"></span>
    <span class="sign none"></span>
    <span class="text none"></span>
  {/if}
{/snippet}

{#snippet line(r: DiffRow, from?: number, block?: number)}
  <div class="row {r.kind}" class:current={block !== undefined && block === at} data-from={from} data-block={block} tabindex="-1">
    {#if r.kind !== 'equal'}<span class="sr-only">{r.kind === 'change' ? 'Changed:' : r.kind === 'delete' ? 'Removed:' : 'Added:'}</span>{/if}
    {@render cell(r.left, 'left', r.kind)}
    {@render cell(r.right, 'right', r.kind)}
  </div>
{/snippet}

<div class="textdiff">
  <div class="bar">
    {#if rows.length === 0}
      <span class="muted">{result.diff ? 'Only line endings differ.' : opts.ignoreSpace || opts.ignoreCase ? 'No differences, apart from what is ignored.' : 'The texts are identical.'}</span>
    {:else}
      {#each parts as p, i (p.what)}
        {#if i}<span class="muted">·</span>{/if}
        <span class={p.cls}>{p.n.toLocaleString()} {p.n === 1 ? 'line' : 'lines'} {p.what}</span>
      {/each}
      {#if first}<span class="muted">· first difference at {first}</span>{/if}
    {/if}
    <div class="spacer"></div>
    {#if blocks.length > 1 && opts.textView === 'split'}
      <span class="muted count">{at < 0 ? blocks.length + ' changes' : `${at + 1} of ${blocks.length}`}</span>
      <IconButton icon={ArrowUp} label="Previous change" shortcut="Alt+↑" size="sm" onclick={() => go(-1)} />
      <IconButton icon={ArrowDown} label="Next change" shortcut="Alt+↓" size="sm" onclick={() => go(1)} />
    {/if}
    {#if result.diff}
      <SegmentedControl
        label="Diff layout"
        options={[
          { value: 'split', label: 'Side by side' },
          { value: 'unified', label: 'Unified' },
        ]}
        bind:value={opts.textView}
      />
      <IconButton icon={Copy} tone="copy" label="Copy as patch" size="sm" onclick={() => copyText(result.diff!, 'Patch')} />
    {/if}
  </div>

  {#if opts.textView === 'unified' && result.diff}
    {#if patch.length > 20000}
      <div class="unified"><CodeView value={result.diff} label="Text diff" readonly /></div>
    {:else}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <pre class="patch" aria-label="Text diff, as a patch" tabindex="0">{#each patch as l, i (i)}<span class="pl {l.kind}">{l.text}
</span>{/each}</pre>
    {/if}
  {:else if rows.length}
    <div class="body">
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div class="rows" bind:this={list} role="region" aria-label="Side-by-side diff" style:--ln={lnWidth} {onkeydown}>
      {#each rows as r, i (i)}
        {#if r.kind !== 'gap'}
          {@render line(r, undefined, blocks.indexOf(i) >= 0 ? blocks.indexOf(i) : undefined)}
        {:else}
          {@const total = r.count ?? 0}
          {@const k = Math.min(opened.get(i) ?? 0, Math.ceil(total / 2))}
          {#if k * 2 >= total}
            {#each unfold(r) as u, j (u.left!.line)}{@render line(u, j === 0 ? i : undefined)}{/each}
          {:else}
            {#each unfold(r, 0, k) as u, j (u.left!.line)}{@render line(u, j === 0 ? i : undefined)}{/each}
            <div class="gap">
              <button type="button" class="more" onclick={() => unfoldAt(i)}>
                <ChevronsUpDown size={12} strokeWidth={1.75} /> {plural(total - 2 * k, total - 2 * k === 1 ? 'unchanged line' : 'unchanged lines')}{total - 2 * k > STEP * 2 ? ` · show ${STEP} more at each end` : ''}
              </button>
              {#if total - 2 * k > STEP * 2}<button type="button" class="all" onclick={() => unfoldAt(i, true)}>Show all</button>{/if}
            </div>
            {#each unfold(r, total - k, total) as u (u.left!.line)}{@render line(u)}{/each}
          {/if}
        {/if}
      {/each}
      {#if result.rowsTruncated}
        <p class="muted more">Showing the first 5,000 rows. Switch to Unified for the whole diff.</p>
      {/if}
    </div>
    {#if marks.length > 1 && scrolls}
      <div class="rail" aria-hidden="true">
        {#each marks as m (m.b)}
          <button type="button" tabindex="-1" class="mark {m.kind}" class:on={m.b === at} style:top="{m.top}%" title="Change {m.b + 1} of {marks.length}" onclick={() => ((at = m.b - 1), go(1))}></button>
        {/each}
      </div>
    {/if}
    </div>
  {/if}
</div>

<style>
  .textdiff {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  /* In a narrow pane the bar wraps between items, never inside one. */
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-2);
    min-height: 36px;
    padding: var(--s-1) var(--s-2) var(--s-1) var(--s-4);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .bar > span {
    white-space: nowrap;
  }
  .spacer {
    flex: 1;
  }
  .count {
    font-variant-numeric: tabular-nums;
  }
  .muted {
    color: var(--fg-2);
  }
  .chg {
    color: var(--warn);
  }
  .del {
    color: var(--err);
  }
  .add {
    color: var(--ok);
  }
  .unified {
    flex: 1;
    min-height: 0;
    display: flex;
  }
  .unified > :global(*) {
    flex: 1;
    min-width: 0;
  }
  /* The rows, and the rail of marks down their right edge. */
  .body {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .rows {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding-right: 10px;
    padding-bottom: var(--s-4);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: 1.6;
  }
  .row {
    position: relative;
    display: grid;
    grid-template-columns: var(--ln) 1.4em 1fr var(--ln) 1.4em 1fr;
  }
  .ln {
    padding-right: var(--s-2);
    text-align: right;
    color: var(--fg-2);
    user-select: none;
  }
  .ln.right {
    border-left: 1px solid var(--border);
  }
  /* The sign of a changed line, so it is not its colour alone. */
  .sign {
    text-align: center;
    color: var(--fg-2);
    user-select: none;
  }
  .sign.tint.left {
    color: var(--err);
    background: var(--diff-del-bg);
  }
  .sign.tint.right {
    color: var(--ok);
    background: var(--diff-add-bg);
  }
  .sign.none {
    background: repeating-linear-gradient(-45deg, transparent 0 4px, var(--bg-2) 4px 5px);
  }
  /* A line changed, not removed and added: one ground on both sides;
     the characters that differ keep the red and the green. */
  .row.change .text.tint,
  .row.change .sign.tint {
    background: var(--diff-chg-bg);
  }
  .row.change .sign.tint {
    color: var(--warn);
  }
  /* The unified view: the patch, coloured. */
  .patch {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: var(--s-2) 0 var(--s-4);
    overflow: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: var(--lh-code);
    color: var(--fg-1);
  }
  .pl {
    display: block;
    padding: 0 var(--s-4);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .pl.add {
    color: var(--fg-0);
    background: var(--diff-add-bg);
  }
  .pl.del {
    color: var(--fg-0);
    background: var(--diff-del-bg);
  }
  .pl.hunk {
    color: var(--accent);
    background: var(--bg-2);
  }
  .pl.file {
    color: var(--fg-2);
  }
  /* Where the changes are, down the right edge. */
  .rail {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    width: 10px;
    background: var(--bg-2);
    border-left: 1px solid var(--border);
  }
  .mark {
    position: absolute;
    left: 1px;
    right: 1px;
    height: 4px;
    min-height: 4px;
    padding: 0;
    border: none;
    border-radius: 1px;
    background: var(--warn);
    cursor: pointer;
  }
  .mark.delete {
    background: var(--err);
  }
  .mark.insert {
    background: var(--ok);
  }
  .mark.on {
    outline: 1px solid var(--fg-0);
  }
  /* Wrap long lines so a whole hash or token stays in view. */
  .text {
    min-width: 0;
    padding-right: var(--s-3);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: var(--fg-1);
  }
  .text.tint.left {
    background: var(--diff-del-bg);
  }
  .text.tint.right {
    background: var(--diff-add-bg);
  }
  .text.none {
    background: repeating-linear-gradient(-45deg, transparent 0 4px, var(--bg-2) 4px 5px);
  }
  .tint {
    color: var(--fg-0);
  }
  mark {
    color: inherit;
    border-radius: 2px;
  }
  .left mark {
    background: var(--diff-del-strong);
  }
  .right mark {
    background: var(--diff-add-strong);
  }
  .gap {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: 0 var(--s-4);
    background: var(--bg-2);
    border-block: 1px solid var(--border);
  }
  .gap button {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    padding: 2px 0;
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    text-align: left;
    background: none;
    border: none;
  }
  .gap .more {
    flex: 1;
  }
  .gap button:hover {
    color: var(--fg-0);
  }
  /* The change Next and Previous went to. */
  .row.current {
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .row:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }
  .gap:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }
  .more {
    padding: var(--s-2) var(--s-4);
    font-family: var(--font-ui);
  }
</style>
