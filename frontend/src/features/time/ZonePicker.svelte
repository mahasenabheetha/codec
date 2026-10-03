<script lang="ts">
  import Globe from '@lucide/svelte/icons/globe'
  import { browserZone, validZone, zoneNames } from './zone.svelte'

  // A time-zone field: type to filter the browser's zone list. Only a
  // real zone is taken; while typing, the last good one stays in use.
  let { value = $bindable(), label = 'Time zone' }: { value: string; label?: string } = $props()

  let text = $state(value)
  const ok = $derived(validZone(text.trim()))
  const listId = `zones-${Math.random().toString(36).slice(2, 8)}`

  $effect(() => {
    text = value // follow changes from outside (another tab)
  })
  function commit() {
    const z = text.trim()
    if (validZone(z)) value = z
  }
</script>

<label class="zone" class:bad={!ok} title={ok ? label : 'Not a time zone; type a name such as Europe/Stockholm'}>
  <Globe size={14} strokeWidth={1.75} />
  <span class="sr-only">{label}</span>
  <input
    bind:value={text}
    list={listId}
    spellcheck="false"
    autocomplete="off"
    oninput={commit}
    onblur={() => (text = value)}
    onkeydown={(e) => e.key === 'Escape' && (text = value)}
  />
  <datalist id={listId}>
    {#each zoneNames as z (z)}
      <option value={z}>{z === browserZone ? 'this machine' : ''}</option>
    {/each}
  </datalist>
</label>

<style>
  .zone {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    height: var(--control-h);
    padding: 0 var(--s-2);
    color: var(--fg-2);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .zone:focus-within {
    border-color: var(--accent);
  }
  .zone.bad {
    border-color: var(--err);
  }
  input {
    width: 170px;
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: transparent;
    border: none;
  }
  input:focus {
    outline: none;
  }
</style>
