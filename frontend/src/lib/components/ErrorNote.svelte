<script lang="ts">
  import CircleX from '@lucide/svelte/icons/circle-x'
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import type { Snippet } from 'svelte'
  import Button from './Button.svelte'

  // Something failed: an icon, the reason in plain words and, where
  // there is one, a way forward (design language: "Error → plain
  // sentence, the reason, a way forward"). One look everywhere, in
  // place of a bare red paragraph. `inline` is the small form for a
  // row or a field; the default is a block inside a panel.
  interface Props {
    message: string
    /** Try the failed thing again. */
    retry?: () => void
    retryLabel?: string
    /** One line beside or under a field or a row. */
    inline?: boolean
    /** Other ways forward, beside Try again. */
    children?: Snippet
  }

  let { message, retry, retryLabel = 'Try again', inline = false, children }: Props = $props()
</script>

<div class="note" class:inline role="alert">
  <CircleX size={inline ? 13 : 15} strokeWidth={2} />
  <span class="msg">{message}</span>
  {#if retry || children}
    <span class="actions">
      {#if retry}<Button size="sm" icon={RotateCw} onclick={retry}>{retryLabel}</Button>{/if}
      {#if children}{@render children()}{/if}
    </span>
  {/if}
</div>

<style>
  .note {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    margin: var(--s-3);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: var(--err-soft);
    border: 1px solid var(--err-border);
    border-radius: var(--r-md);
  }
  .note > :global(svg) {
    flex: 0 0 auto;
    margin-top: 3px;
    color: var(--err);
  }
  .msg {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
  .actions {
    flex: 0 0 auto;
    display: flex;
    gap: var(--s-2);
  }
  .inline {
    margin: var(--s-1) 0 0;
    padding: 0;
    font-size: var(--fs-sm);
    color: var(--err);
    background: none;
    border: none;
  }
  .inline > :global(svg) {
    margin-top: 2px;
  }
</style>
