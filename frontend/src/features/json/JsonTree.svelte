<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import Copy from '@lucide/svelte/icons/copy'
  import { copyText } from '../../lib/utils/clipboard'
  import { untrack } from 'svelte'
  import JsonTree from './JsonTree.svelte'

  // One node of the JSON tree, and its children when open: the key, a
  // preview or count, and a button to copy its path (jq style). Big
  // arrays and objects show their first PAGE children, then "more".
  interface Props {
    value: unknown
    name?: string
    path?: string
    depth?: number
    /** Open every level (Open everything); else only the top one. */
    all?: boolean
  }

  let { value, name, path = '.', depth = 0, all = false }: Props = $props()
  let expanded = $state(untrack(() => all || depth < 1))
  const PAGE = 200
  let shown = $state(PAGE)

  const isObj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v)
  const kids = $derived<[string, unknown, string][]>(
    Array.isArray(value)
      ? value.map((v, i) => [String(i), v, `${path === '.' ? '' : path}[${i}]`])
      : isObj(value)
        ? Object.entries(value).map(([k, v]) => [k, v, `${path === '.' ? '' : path}${/^[A-Za-z_][A-Za-z0-9_]*$/.test(k) ? '.' + k : `[${JSON.stringify(k)}]`}`])
        : [],
  )
  const container = $derived(Array.isArray(value) || isObj(value))
  const kind = $derived(value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value)
  const preview = $derived.by(() => {
    if (Array.isArray(value)) return `[${value.length}]`
    if (isObj(value)) return `{${Object.keys(value).length}}`
    return JSON.stringify(value)
  })
</script>

<div class="node">
  <div class="row" style:padding-left="{depth * 16}px">
    {#if container}
      <button type="button" class="twist" class:open={expanded} aria-expanded={expanded} aria-label={expanded ? 'Fold' : 'Unfold'} onclick={() => (expanded = !expanded)}>
        <ChevronRight size={14} strokeWidth={2} />
      </button>
    {:else}
      <span class="twist-space"></span>
    {/if}
    {#if name !== undefined}<span class="key" class:index={Array.isArray(value) === false && /^\d+$/.test(name)}>{name}</span><span class="colon">:</span>{/if}
    <span class="val {kind}" class:count={container}>{preview}</span>
    <button type="button" class="copy" title="Copy the path {path}" aria-label="Copy the path {path}" onclick={() => copyText(path, 'Path')}><Copy size={12} strokeWidth={2} /></button>
  </div>
  {#if container && expanded}
    {#each kids.slice(0, shown) as [k, v, p] (p)}
      <JsonTree value={v} name={k} path={p} depth={depth + 1} {all} />
    {/each}
    {#if kids.length > shown}
      <button type="button" class="more" style:margin-left="{(depth + 1) * 16 + 20}px" onclick={() => (shown += PAGE)}>
        Show {Math.min(PAGE, kids.length - shown)} more of {kids.length - shown}
      </button>
    {/if}
  {/if}
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    min-height: 24px;
    padding-right: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    border-radius: var(--r-sm);
  }
  .row:hover {
    background: var(--bg-3);
  }
  .twist,
  .twist-space {
    display: grid;
    place-items: center;
    flex: none;
    width: 18px;
    height: 18px;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
  }
  .twist :global(svg) {
    transition: transform var(--dur) var(--ease);
  }
  .twist.open :global(svg) {
    transform: rotate(90deg);
  }
  .key {
    color: var(--syn-key);
  }
  .colon {
    color: var(--syn-punct);
    margin-right: var(--s-1);
  }
  .val {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .val.string {
    color: var(--syn-string);
  }
  .val.number {
    color: var(--syn-number);
  }
  .val.boolean,
  .val.null {
    color: var(--syn-bool);
  }
  .val.count {
    color: var(--fg-2);
  }
  .copy {
    display: grid;
    place-items: center;
    flex: none;
    width: 20px;
    height: 20px;
    margin-left: auto;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
  }
  .row:hover .copy,
  .copy:focus-visible {
    opacity: 1;
  }
  .copy:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
  .more {
    padding: 2px var(--s-2);
    font-size: var(--fs-sm);
    color: var(--accent);
    background: none;
    border: none;
  }
</style>
