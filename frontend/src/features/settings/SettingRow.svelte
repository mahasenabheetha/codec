<script lang="ts">
  import type { Snippet } from 'svelte'

  // One setting: what it is and why on the left, its control on the right.
  interface Props {
    title: string
    description?: string
    /** id of the control, so clicking the title focuses it. */
    for?: string
    /** Extra content under the description (e.g. code samples). */
    details?: Snippet
    children: Snippet
  }

  let { title, description, for: forId, details, children }: Props = $props()
</script>

<div class="row">
  <svelte:element this={forId ? 'label' : 'div'} class="label" for={forId}>
    <span class="title">{title}</span>
    {#if description}<small>{description}</small>{/if}
    {#if details}<small>{@render details()}</small>{/if}
  </svelte:element>
  <div class="control">{@render children()}</div>
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-4);
    padding: var(--s-3) 0;
    border-top: 1px solid var(--border);
  }
  .label {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: var(--fs-md);
  }
  small {
    font-size: var(--fs-sm);
    color: var(--fg-1);
    line-height: 1.4;
  }
  .control {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    flex-shrink: 0;
  }
  @media (max-width: 720px) {
    .row {
      flex-wrap: wrap;
    }
  }
</style>
