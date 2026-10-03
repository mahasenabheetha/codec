<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import Copy from '@lucide/svelte/icons/copy'
  import CodeView from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { sizeLabel } from '../shared/detect'
  import type { ToolError } from '../shared/transform.svelte'

  // The output pane of a text-to-text tab: result, error or empty
  // state, with Swap and Copy; extra content (a parts table) below.
  interface Props {
    output: string | null
    error: ToolError | null
    busy: boolean
    emptyIcon: Component
    emptyTitle: string
    emptyDescription: string
    onsample?: () => void
    onswap?: () => void
    badges?: Snippet
    children?: Snippet
  }

  let { output, error, busy, emptyIcon, emptyTitle, emptyDescription, onsample, onswap, badges, children }: Props = $props()
</script>

<Pane title="Output">
  {#snippet meta()}
    {#if badges}{@render badges()}{/if}
    {#if output !== null}<span>{sizeLabel(output)}</span>{/if}
    {#if busy}<span class="busy" aria-label="Working"></span>{/if}
  {/snippet}
  {#snippet actions()}
    {#if onswap}
      <IconButton icon={ArrowLeftRight} label="Use output as input" shortcut="Alt+S" size="sm" disabled={!output} onclick={onswap} />
    {/if}
    <IconButton icon={Copy} label="Copy output" shortcut="Alt+C" size="sm" disabled={!output} onclick={() => copyText(output ?? '')} />
  {/snippet}

  {#if error}
    <ErrorBanner {error} />
  {/if}
  {#if output !== null}
    <div class="stack">
      <div class="code"><CodeView value={output} label="Output" readonly wrap /></div>
      {#if children}{@render children()}{/if}
    </div>
  {:else if !error}
    <EmptyState icon={emptyIcon} title={emptyTitle} description={emptyDescription}>
      {#if onsample}<Button size="sm" onclick={onsample}>Try a sample</Button>{/if}
    </EmptyState>
  {/if}
</Pane>

<style>
  .stack {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .code {
    flex: 1;
    min-height: 80px;
    display: flex;
    flex-direction: column;
  }
  /* As in OutputPane: hidden for the first 250ms so quick jobs don't flash. */
  .busy {
    width: 12px;
    height: 12px;
    border: 2px solid var(--bg-4);
    border-top-color: var(--accent);
    border-radius: 50%;
    opacity: 0;
    animation:
      spin 0.7s linear infinite,
      appear 1ms 250ms forwards;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @keyframes appear {
    to {
      opacity: 1;
    }
  }
</style>
