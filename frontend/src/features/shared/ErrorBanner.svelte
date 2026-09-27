<script lang="ts">
  import CircleX from '@lucide/svelte/icons/circle-x'
  import type { ToolError } from './transform.svelte'

  // Shows a transform error. When the error has a position, the whole
  // banner is a button that jumps to it (like v1's click-to-jump).
  interface Props {
    error: ToolError
    onjump?: (line: number, column: number) => void
  }

  let { error, onjump }: Props = $props()
  const jumpable = $derived(error.line !== undefined && !!onjump)
</script>

{#if jumpable}
  <button type="button" class="banner" onclick={() => onjump?.(error.line!, error.column ?? 1)}>
    <CircleX size={16} strokeWidth={2} />
    <span class="msg">{error.message}</span>
    <span class="jump">Jump to line {error.line}, col {error.column ?? 1}</span>
  </button>
{:else}
  <div class="banner" role="alert">
    <CircleX size={16} strokeWidth={2} />
    <span class="msg">{error.message}</span>
  </div>
{/if}

<style>
  .banner {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    width: 100%;
    margin: 0;
    padding: var(--s-3);
    font-size: var(--fs-md);
    text-align: left;
    color: var(--fg-0);
    background: var(--err-soft);
    border: none;
    border-bottom: 1px solid var(--err-border);
  }
  .banner :global(svg) {
    flex: 0 0 auto;
    margin-top: 2px;
    color: var(--err);
  }
  .msg {
    flex: 1;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: var(--lh-code);
    word-break: break-word;
  }
  .jump {
    flex: 0 0 auto;
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--err);
  }
  button.banner:hover .jump {
    text-decoration: underline;
  }
</style>
