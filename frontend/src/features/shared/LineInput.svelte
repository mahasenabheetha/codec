<script lang="ts">
  import type { Snippet } from 'svelte'
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import Eraser from '@lucide/svelte/icons/eraser'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { toast } from '../../lib/stores/toast.svelte'

  // A one-line input for tools whose input is a value, not a document
  // (a timestamp, a cron expression): large, monospaced, with paste and
  // clear. Extra content (a field breakdown) goes below it.
  interface Props {
    value: string
    label: string
    placeholder?: string
    invalid?: boolean
    onclear?: () => void
    actions?: Snippet
    children?: Snippet
    field?: HTMLInputElement
  }

  let { value = $bindable(), label, placeholder, invalid = false, onclear, actions, children, field = $bindable() }: Props = $props()

  async function paste() {
    try {
      value = (await navigator.clipboard.readText()).trim()
      field?.focus()
    } catch {
      toast('Clipboard access was blocked — paste with Ctrl+V instead', 'warn')
    }
  }
</script>

<section class="line" aria-label={label}>
  <div class="row" class:invalid>
    <input bind:this={field} bind:value aria-label={label} {placeholder} spellcheck="false" autocomplete="off" />
    {#if actions}{@render actions()}{/if}
    <IconButton icon={ClipboardPaste} label="Paste from clipboard" size="sm" onclick={paste} />
    <IconButton icon={Eraser} label="Clear" shortcut="Escape" size="sm" disabled={!value} onclick={() => (onclear ? onclear() : (value = ''))} />
  </div>
  {#if children}{@render children()}{/if}
</section>

<style>
  .line {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding: var(--s-3);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 var(--s-1) 0 var(--s-3);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
  }
  .row:focus-within {
    border-color: var(--accent);
  }
  .row.invalid {
    border-color: var(--err);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 40px;
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-xl);
    letter-spacing: 0.02em;
    color: var(--fg-0);
    background: transparent;
    border: none;
  }
  input:focus {
    outline: none;
  }
</style>
