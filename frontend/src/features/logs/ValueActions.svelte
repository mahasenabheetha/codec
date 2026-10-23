<script lang="ts">
  import Columns3 from '@lucide/svelte/icons/columns-3'
  import Copy from '@lucide/svelte/icons/copy'
  import Footprints from '@lucide/svelte/icons/footprints'
  import FunnelPlus from '@lucide/svelte/icons/funnel-plus'
  import FunnelX from '@lucide/svelte/icons/funnel-x'
  import { copyText } from '../../lib/utils/clipboard'
  import { maskSecrets } from './mask'
  import { followable, nameable, term } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // What can be done with one value: filter to it or out, show its
  // field as a column, follow an ID across the log, copy it.
  interface Props {
    key: string // the field, or a short name (source, logger, trace)
    value: string
    column?: boolean // the key is a field that can be a column
  }

  let { key, value, column = false }: Props = $props()
  const viewer = useViewer()

  const can = $derived(nameable(key))
  const follow = $derived(can && (followable(key) || key === 'trace'))

  function copy() {
    const m = maskSecrets(`${key}=${value}`)
    copyText(m.masked ? m.text.slice(key.length + 1) : value, m.masked ? 'Value (masked)' : 'Value')
  }
</script>

<span class="actions">
  {#if can}
    <button type="button" title="Only entries with this value" aria-label="Only this" onclick={() => viewer.addTerm(term(key, value))}><FunnelPlus size={12} /></button>
    <button type="button" title="Hide entries with this value" aria-label="Hide this" onclick={() => viewer.addTerm(term(key, value, true))}><FunnelX size={12} /></button>
  {/if}
  {#if follow}
    <button type="button" title="Follow: every entry with this {key}, whatever else is filtered" aria-label="Follow" onclick={() => viewer.follow(term(key, value))}><Footprints size={12} /></button>
  {/if}
  {#if column && can}
    <button
      type="button"
      class:on={prefs.columns.value.includes(key)}
      title={prefs.columns.value.includes(key) ? 'Remove the column' : 'Show as a column'}
      aria-label="Column"
      onclick={() => viewer.toggleColumn(key)}><Columns3 size={12} /></button
    >
  {/if}
  <button type="button" title="Copy the value" aria-label="Copy" onclick={copy}><Copy size={12} /></button>
</span>

<style>
  .actions {
    display: inline-flex;
    gap: 1px;
    opacity: 0;
    transition: opacity 0.1s;
  }
  :global(:hover) > .actions,
  :global(:focus-within) > .actions {
    opacity: 1;
  }
  button {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  button:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
  button.on {
    color: var(--accent);
  }
</style>
