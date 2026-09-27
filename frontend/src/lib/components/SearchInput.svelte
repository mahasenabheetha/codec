<script lang="ts">
  import Search from '@lucide/svelte/icons/search'
  import X from '@lucide/svelte/icons/x'

  interface Props {
    value: string
    placeholder?: string
    label?: string
  }

  let { value = $bindable(), placeholder = 'Filter…', label = 'Filter' }: Props = $props()
</script>

<div class="search">
  <Search size={14} strokeWidth={1.75} />
  <input type="search" bind:value {placeholder} aria-label={label} spellcheck="false" />
  {#if value}
    <button type="button" aria-label="Clear filter" onclick={() => (value = '')}>
      <X size={12} strokeWidth={2} />
    </button>
  {/if}
</div>

<style>
  .search {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 var(--s-2);
    color: var(--fg-2);
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    transition: border-color var(--dur) var(--ease);
  }
  .search:focus-within {
    border-color: var(--accent);
  }
  input {
    width: 160px;
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: none;
    border: none;
    outline: none;
  }
  input::placeholder {
    color: var(--fg-2);
  }
  input::-webkit-search-cancel-button {
    display: none;
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
