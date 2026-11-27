<script lang="ts">
  import type { Snippet } from 'svelte'
  import Search from '@lucide/svelte/icons/search'
  import X from '@lucide/svelte/icons/x'

  interface Props {
    value: string
    placeholder?: string
    label?: string
    /** The one search a page is built around (Home's): taller, bigger
     *  text, the icon in the accent, and as wide as its box. */
    large?: boolean
    /** After the field, e.g. the shortcut that does more. */
    end?: Snippet
  }

  let { value = $bindable(), placeholder = 'Filter…', label = 'Filter', large = false, end }: Props = $props()
</script>

<div class="search" class:large>
  <Search size={large ? 16 : 14} />
  <input type="search" bind:value {placeholder} aria-label={label} spellcheck="false" />
  {#if value}
    <button type="button" aria-label="Clear filter" onclick={() => (value = '')}>
      <X size={12} />
    </button>
  {/if}
  {#if end}{@render end()}{/if}
</div>

<style>
  .search {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 var(--s-2);
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    transition:
      border-color var(--dur) var(--ease),
      box-shadow var(--dur) var(--ease);
  }
  .search:hover {
    border-color: var(--fg-2);
  }
  .search:focus-within {
    border-color: var(--tool-accent, var(--accent));
    box-shadow: var(--focus-ring);
  }
  .search:focus-within > :global(svg) {
    color: var(--tool-accent, var(--accent));
  }
  input {
    width: 160px;
    min-width: 0;
    height: 100%;
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: none;
    border: none;
    box-shadow: none;
    outline: none;
  }
  input::placeholder {
    color: var(--fg-2);
  }
  input::-webkit-search-cancel-button {
    display: none;
  }
  .large {
    height: calc(var(--control-h) + var(--s-3));
    padding: 0 var(--s-3);
    gap: var(--s-3);
    border-radius: var(--r-lg);
  }
  .large > :global(svg:first-child) {
    color: var(--tool-accent, var(--accent));
  }
  .large input {
    flex: 1;
    width: auto;
    font-size: var(--fs-lg);
  }
  button {
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    color: var(--fg-1);
    background: var(--bg-3);
    border: none;
    border-radius: 50%;
  }
</style>
