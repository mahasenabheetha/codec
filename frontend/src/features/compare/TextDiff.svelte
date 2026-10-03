<script lang="ts">
  import { tick } from 'svelte'
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down'
  import Copy from '@lucide/svelte/icons/copy'
  import CodeView from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import type { CompareResult, DiffCell, DiffRow } from '../../lib/api/compare'
  import { copyText } from '../../lib/utils/clipboard'
  import { comparison as cmp } from './compare.svelte'

  // A text diff side by side: each changed line beside the line that
  // replaced it, the changed characters marked strongly, unchanged runs
  // folded into gaps that open on click. Unified shows the patch.
  interface Props {
    result: CompareResult
  }

  let { result }: Props = $props()

  const rows = $derived(result.rows ?? [])
  // Gaps opened by the user, by row index. CompareView keys this
  // component on the result, so a new result starts folded.
  let opened = $state(new Set<number>())
  let list = $state<HTMLElement>()

  async function unfoldAt(i: number) {
    opened = new Set(opened).add(i)
    await tick()
    list?.querySelector<HTMLElement>(`[data-from="${i}"]`)?.focus()
  }

  const lines = $derived({ left: result.left.text.split(/\r?\n/), right: result.right.text.split(/\r?\n/) })

  /** A gap's unchanged lines, as equal rows. */
  function unfold(r: DiffRow): DiffRow[] {
    const out: DiffRow[] = []
    for (let k = 0; k < (r.count ?? 0); k++) {
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
</script>

{#snippet cell(c: DiffCell | undefined, side: 'left' | 'right', kind: DiffRow['kind'])}
  {#if c}
    <span class="ln">{#if kind !== 'equal'}<span class="sr-only">{side === 'left' ? 'old' : 'new'} </span>{/if}{c.line}</span>
    <span class="text {side}" class:tint={kind !== 'equal'}>{#each pieces(c) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>
  {:else}
    <span class="ln"></span>
    <span class="text none"></span>
  {/if}
{/snippet}

{#snippet line(r: DiffRow, from?: number)}
  <div class="row {r.kind}" data-from={from} tabindex="-1">
    {#if r.kind !== 'equal'}<span class="sr-only">{r.kind === 'change' ? 'Changed:' : r.kind === 'delete' ? 'Removed:' : 'Added:'}</span>{/if}
    {@render cell(r.left, 'left', r.kind)}
    {@render cell(r.right, 'right', r.kind)}
  </div>
{/snippet}

<div class="textdiff">
  <div class="bar">
    {#if rows.length === 0}
      <span class="muted">{result.diff ? 'Only line endings differ.' : cmp.ignoreSpace || cmp.ignoreCase ? 'No differences, apart from what is ignored.' : 'The texts are identical.'}</span>
    {:else}
      {#each parts as p, i (p.what)}
        {#if i}<span class="muted">·</span>{/if}
        <span class={p.cls}>{p.n.toLocaleString()} {p.n === 1 ? 'line' : 'lines'} {p.what}</span>
      {/each}
      {#if first}<span class="muted">· first difference at {first}</span>{/if}
    {/if}
    <div class="spacer"></div>
    {#if result.diff}
      <SegmentedControl
        label="Diff layout"
        options={[
          { value: 'split', label: 'Side by side' },
          { value: 'unified', label: 'Unified' },
        ]}
        bind:value={cmp.textView}
      />
      <IconButton icon={Copy} label="Copy as patch" size="sm" onclick={() => copyText(result.diff!, 'Patch')} />
    {/if}
  </div>

  {#if cmp.textView === 'unified' && result.diff}
    <div class="unified"><CodeView value={result.diff} label="Text diff" readonly /></div>
  {:else if rows.length}
    <div class="rows" bind:this={list} role="region" aria-label="Side-by-side diff" style:--ln={lnWidth}>
      {#each rows as r, i (i)}
        {#if r.kind !== 'gap'}
          {@render line(r)}
        {:else if opened.has(i)}
          {#each unfold(r) as u, k (u.left!.line)}{@render line(u, k === 0 ? i : undefined)}{/each}
        {:else}
          <button type="button" class="gap" onclick={() => unfoldAt(i)}>
            <ChevronsUpDown size={12} strokeWidth={1.75} /> {plural(r.count ?? 0, r.count === 1 ? 'unchanged line' : 'unchanged lines')}
          </button>
        {/if}
      {/each}
      {#if result.rowsTruncated}
        <p class="muted more">Showing the first 5,000 rows. Switch to Unified for the whole diff.</p>
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
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: 36px;
    padding: var(--s-1) var(--s-2) var(--s-1) var(--s-4);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .spacer {
    flex: 1;
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
  .rows {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding-bottom: var(--s-4);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: 1.6;
  }
  .row {
    position: relative;
    display: grid;
    grid-template-columns: var(--ln) 1fr var(--ln) 1fr;
  }
  .ln {
    padding-right: var(--s-2);
    text-align: right;
    color: var(--fg-2);
    user-select: none;
  }
  .ln:nth-child(3) {
    border-left: 1px solid var(--border);
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
    gap: var(--s-2);
    width: 100%;
    padding: 2px var(--s-4);
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    text-align: left;
    background: var(--bg-2);
    border: none;
    border-block: 1px solid var(--border);
  }
  .gap:hover {
    color: var(--fg-0);
    background: var(--bg-3);
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
