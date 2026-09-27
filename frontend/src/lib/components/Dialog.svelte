<script lang="ts">
  import { Dialog } from 'bits-ui'
  import X from '@lucide/svelte/icons/x'
  import type { Snippet } from 'svelte'

  // Modal dialog: focus trap, Escape to close and scroll lock come from
  // Bits UI; this wrapper only styles it.
  interface Props {
    open: boolean
    title: string
    description?: string
    width?: string
    children: Snippet
  }

  let { open = $bindable(), title, description, width = '520px', children }: Props = $props()
</script>

<Dialog.Root bind:open>
  <Dialog.Portal>
    <Dialog.Overlay class="codec-overlay" />
    <Dialog.Content class="codec-dialog" style="width: min({width}, calc(100vw - 32px))">
      <header>
        <div>
          <Dialog.Title class="codec-dialog-title">{title}</Dialog.Title>
          {#if description}
            <Dialog.Description class="codec-dialog-desc">{description}</Dialog.Description>
          {/if}
        </div>
        <Dialog.Close class="codec-dialog-close" aria-label="Close">
          <X size={16} strokeWidth={1.75} />
        </Dialog.Close>
      </header>
      <div class="body">{@render children()}</div>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  :global(.codec-overlay) {
    position: fixed;
    inset: 0;
    z-index: var(--z-dialog);
    background: var(--overlay);
    animation: overlay-in var(--dur) var(--ease);
  }
  :global(.codec-dialog) {
    position: fixed;
    top: 14vh;
    left: 50%;
    transform: translateX(-50%);
    z-index: var(--z-dialog);
    max-height: 72vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-1);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-pop);
    animation: dialog-in 160ms var(--ease);
  }
  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--s-4);
    padding: var(--s-4) var(--s-4) var(--s-3) var(--s-5);
  }
  :global(.codec-dialog-title) {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
  }
  :global(.codec-dialog-desc) {
    margin-top: var(--s-1);
    color: var(--fg-1);
  }
  :global(.codec-dialog-close) {
    display: grid;
    place-items: center;
    width: var(--control-h);
    height: var(--control-h);
    padding: 0;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
  }
  :global(.codec-dialog-close:hover) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .body {
    padding: 0 var(--s-5) var(--s-5);
    overflow: auto;
  }
  @keyframes overlay-in {
    from {
      opacity: 0;
    }
  }
  @keyframes dialog-in {
    from {
      opacity: 0;
      transform: translate(-50%, 6px) scale(0.98);
    }
  }
</style>
