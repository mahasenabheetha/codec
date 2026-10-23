<script lang="ts">
  import Columns3 from '@lucide/svelte/icons/columns-3'
  import FunnelPlus from '@lucide/svelte/icons/funnel-plus'
  import FunnelX from '@lucide/svelte/icons/funnel-x'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import { isAbort } from '../../lib/api/client'
  import { logFields, type FieldStat } from '../../lib/api/logs'
  import { count } from './format'
  import { nameable, term } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // Every field of the matching entries: how often it is there, its
  // most common values (a click filters to one), and for numbers the
  // range, mean, median and 95th percentile.
  interface Props {
    onshow: () => void
  }

  let { onshow }: Props = $props()
  const viewer = useViewer()

  let fields = $state.raw<FieldStat[] | null>(null)
  let total = $state(0)
  let error = $state<string | null>(null)
  let filter = $state('')
  $effect(() => {
    const q = { ...viewer.applied }
    void viewer.id
    const c = new AbortController()
    error = null
    viewer
      .withLog((id) => logFields(id, { ...q, top: 8 }, c.signal))
      .then((r) => {
        fields = r.fields
        total = r.total
      })
      .catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    return () => c.abort()
  })

  const shown = $derived((fields ?? []).filter((f) => f.key.toLowerCase().includes(filter.trim().toLowerCase())))
  const num = (n: number) => (Number.isInteger(n) ? count(n) : n.toPrecision(4).replace(/\.?0+$/, ''))

  function only(key: string, value: string, hide = false) {
    viewer.addTerm(term(key, value, hide))
    if (!hide) onshow()
  }
</script>

<div class="fields">
  <div class="bar">
    <span class="muted">{fields ? `${count(fields.length)} fields in ${count(total)} entries` : 'Reading fields…'}</span>
    <div class="search"><SearchInput bind:value={filter} placeholder="Find a field" /></div>
  </div>
  {#if error}
    <p class="err">{error}</p>
  {:else if fields && !fields.length}
    <p class="muted pad">These entries have no fields, only text.</p>
  {:else}
    <div class="list">
      {#each shown as f (f.key)}
        <section>
          <header>
            <span class="key">{f.key}</span>
            <span class="muted">{Math.round((f.count / Math.max(total, 1)) * 100)}% · {count(f.distinct)}{f.more ? '+' : ''} {f.distinct === 1 ? 'value' : 'values'}</span>
            {#if f.key !== 'source' && nameable(f.key)}
              <button type="button" class="col" class:on={prefs.columns.value.includes(f.key)} title="Show as a column" onclick={() => viewer.toggleColumn(f.key)}><Columns3 size={12} /></button>
            {/if}
          </header>
          {#if f.num}
            <p class="num">min {num(f.num.min)} · median {num(f.num.median)} · p95 {num(f.num.p95)} · max {num(f.num.max)} · mean {num(f.num.mean)}</p>
          {/if}
          {#if f.long}
            <p class="muted small">Long text (stack traces, statements): values aren't counted.</p>
          {:else}
            <ul>
              {#each f.top ?? [] as v (v.value)}
                <li>
                  <span class="bar-fill" style:width="{(v.count / f.count) * 100}%"></span>
                  <button type="button" class="val" title="Only entries with this value" onclick={() => only(f.key, v.value)}>{v.value === '' ? '(empty)' : v.value}</button>
                  <span class="cnt">{count(v.count)}</span>
                  <button type="button" class="hide" title="Hide entries with this value" aria-label="Hide this" onclick={() => only(f.key, v.value, true)}><FunnelX size={11} /></button>
                  <button type="button" class="hide" title="Only entries with this value" aria-label="Only this" onclick={() => only(f.key, v.value)}><FunnelPlus size={11} /></button>
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      {/each}
    </div>
  {/if}
</div>

<style>
  .fields {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
    font-size: var(--fs-sm);
  }
  .search {
    width: 220px;
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    align-content: start;
    gap: var(--s-3);
    padding: var(--s-3);
  }
  section {
    min-width: 0;
    padding: var(--s-2) var(--s-3);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  header {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
  }
  .key {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--syn-key);
  }
  .muted {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-3);
  }
  .small {
    margin: var(--s-1) 0 0;
  }
  .num {
    margin: var(--s-1) 0 0;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--syn-number);
  }
  ul {
    margin: var(--s-1) 0 0;
    padding: 0;
    list-style: none;
  }
  li {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: 1px var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .bar-fill {
    position: absolute;
    inset: 0 auto 0 0;
    background: var(--accent-soft);
    border-radius: var(--r-sm);
    pointer-events: none;
  }
  .val {
    position: relative;
    flex: 1;
    min-width: 0;
    padding: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .val:hover {
    text-decoration: underline;
  }
  .cnt {
    position: relative;
    color: var(--fg-1);
    font-variant-numeric: tabular-nums;
  }
  .hide,
  .col {
    position: relative;
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
  }
  li:hover .hide,
  .col,
  .hide:focus-visible {
    opacity: 1;
  }
  .hide:hover,
  .col:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
  .col.on {
    color: var(--accent);
  }
  .err {
    padding: var(--s-3);
    color: var(--err);
  }
</style>
