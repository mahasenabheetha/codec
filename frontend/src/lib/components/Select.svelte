<script lang="ts">
  import { Select } from 'bits-ui'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import Check from '@lucide/svelte/icons/check'

  // Compact single-choice dropdown (listbox semantics via Bits UI).
  interface Item {
    value: string
    label: string
    /** Colours shown before the label: a ground and dots on it (a
     *  theme's swatch). */
    swatch?: string[]
    /** Quieter words after the label. */
    note?: string
  }

  interface Props {
    items: Item[]
    value: string
    label: string
    disabled?: boolean
    onchange?: (value: string) => void
  }

  let { items, value = $bindable(), label, disabled = false, onchange }: Props = $props()
  const selected = $derived(items.find((i) => i.value === value))
  const selectedLabel = $derived(selected?.label ?? label)
</script>

<Select.Root type="single" bind:value {items} {disabled} onValueChange={(v) => onchange?.(v)}>
  <Select.Trigger class="codec-select-trigger" aria-label={label}>
    {#if selected?.swatch}{@render chip(selected.swatch)}{/if}
    <span>{selectedLabel}</span>
    <ChevronDown size={14} strokeWidth={1.75} />
  </Select.Trigger>
  <Select.Portal>
    <Select.Content class="codec-select-content" sideOffset={4}>
      <Select.Viewport>
        {#each items as item (item.value)}
          <Select.Item value={item.value} label={item.label} class="codec-select-item">
            {#snippet children({ selected })}
              <span class="codec-select-text">
                {#if item.swatch}{@render chip(item.swatch)}{/if}
                <span>{item.label}</span>
                {#if item.note}<span class="codec-select-note">{item.note}</span>{/if}
              </span>
              {#if selected}<Check size={14} strokeWidth={2} />{/if}
            {/snippet}
          </Select.Item>
        {/each}
      </Select.Viewport>
    </Select.Content>
  </Select.Portal>
</Select.Root>

{#snippet chip(colours: string[])}
  <span class="codec-swatch" style:background={colours[0]} aria-hidden="true">
    {#each colours.slice(1) as c, i (i)}<span style:background={c}></span>{/each}
  </span>
{/snippet}

<style>
  :global(.codec-swatch) {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 0 4px;
    height: 16px;
    border-radius: var(--r-sm);
    box-shadow: inset 0 0 0 1px var(--border-strong);
    flex: none;
  }
  :global(.codec-swatch span) {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  :global(.codec-select-text) {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
  }
  :global(.codec-select-note) {
    color: var(--fg-2);
    font-size: var(--fs-xs);
  }
  :global(.codec-select-trigger) {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 var(--s-2) 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
  }
  :global(.codec-select-trigger:hover:not(:disabled)) {
    background: var(--bg-4);
  }
  :global(.codec-select-trigger:disabled) {
    opacity: 0.45;
    cursor: default;
  }
  :global(.codec-select-content) {
    z-index: var(--z-popover);
    min-width: var(--bits-select-anchor-width);
    padding: var(--s-1);
    background: var(--bg-2);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-pop);
  }
  :global(.codec-select-item) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    height: 28px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    border-radius: var(--r-sm);
    cursor: pointer;
    outline: none;
  }
  :global(.codec-select-item[data-highlighted]) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  :global(.codec-select-item[data-selected]) {
    color: var(--fg-0);
  }
</style>
