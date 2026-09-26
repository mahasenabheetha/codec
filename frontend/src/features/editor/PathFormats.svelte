<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import { pathAt, type PathInfo } from '../../lib/api/yaml'
  import { copyText } from '../../lib/utils/clipboard'
  import type { FileSession } from './session.svelte'

  // The path at the cursor in every notation the engine knows; click
  // one to copy it. Fetched when shown, for the cursor at that moment.
  let { session }: { session: FileSession } = $props()

  const labels: Record<string, string> = {
    dot: 'Dot',
    yq: 'yq',
    jsonpath: 'JSONPath',
    helm: 'Helm',
    set: 'helm --set',
  }

  let info = $state<PathInfo | null>(null)
  let error = $state('')

  $effect(() => {
    const { line, col } = session.cursor
    pathAt({ path: session.path, content: session.content(), line, col })
      .then((r) => (info = r))
      .catch((e) => (error = e.message))
  })

  const order = ['dot', 'yq', 'jsonpath', 'helm', 'set'] as const
  const rows = $derived(order.filter((k) => info?.formats[k]).map((k) => [k, info!.formats[k]] as const))
</script>

<div class="formats">
  <div class="head">Copy path at line {session.cursor.line}</div>
  {#if error}
    <p class="msg err">{error}</p>
  {:else if info && rows.length === 0}
    <p class="msg">No key or item at the cursor.</p>
  {:else}
    {#each rows as [style, value] (style)}
      <button type="button" class="row" onclick={() => copyText(value!, labels[style] + ' path')}>
        <span class="label">{labels[style] ?? style}</span>
        <code>{value}</code>
        <Copy size={12} strokeWidth={1.75} />
      </button>
    {/each}
  {/if}
</div>

<style>
  .formats {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 320px;
    max-width: 560px;
  }
  .head {
    padding: 0 var(--s-1) var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .row {
    display: grid;
    grid-template-columns: 80px 1fr auto;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    text-align: left;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .row:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  code {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--syn-key);
    word-break: break-all;
  }
  .msg {
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .err {
    color: var(--err);
  }
</style>
