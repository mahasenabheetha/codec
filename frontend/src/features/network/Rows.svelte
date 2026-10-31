<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { copyText } from '../../lib/utils/clipboard'

  // Name and value rows, each value with Copy; empty values are left out.
  // copy is the text Copy takes when it differs from what is shown
  // (a count shown with separators is copied without).
  interface Row {
    label: string
    value?: string
    copy?: string
  }

  let { rows }: { rows: Row[] } = $props()

  const shown = $derived(rows.filter((r) => r.value))
</script>

<dl class="rows">
  {#each shown as r (r.label)}
    <div class="row">
      <dt>{r.label}</dt>
      <dd>
        <code>{r.value}</code>
        <IconButton icon={Copy} label="Copy {r.label}" size="sm" onclick={() => copyText(r.copy ?? r.value!, r.label)} />
      </dd>
    </div>
  {/each}
</dl>

<style>
  .rows {
    margin: 0;
    padding: var(--s-2) var(--s-3);
  }
  .row {
    display: grid;
    grid-template-columns: minmax(120px, 180px) 1fr;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-1) 0;
    border-bottom: 1px solid var(--border);
  }
  .row:last-child {
    border-bottom: none;
  }
  dt {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  /* The copy button sits right after its value, not at the far edge. */
  dd {
    margin: 0;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  code {
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
</style>
