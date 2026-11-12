<script lang="ts">
  import Working from '../../lib/components/Working.svelte'
  import type { Component, Snippet } from 'svelte'
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import Copy from '@lucide/svelte/icons/copy'
  import Search from '@lucide/svelte/icons/search'
  import CodeView, { type Language } from '../../lib/components/CodeView.svelte'
  import FindBar from './FindBar.svelte'
  import { findShortcut } from './find.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from './Pane.svelte'
  import ErrorBanner from './ErrorBanner.svelte'
  import { displayLanguage, sizeLabel } from './detect'
  import type { ToolError } from './transform.svelte'

  // The output pane of every tool: result, error or empty state, with
  // Swap, Find and Copy; extra content (a parts table) below. The one
  // implementation: OutputPane is this, fed from a transform session.
  interface Props {
    output: string | null
    error: ToolError | null
    busy: boolean
    emptyIcon: Component
    emptyTitle: string
    emptyDescription: string
    onsample?: () => void
    onswap?: () => void
    /** Jump to an error's position in the input. */
    onjump?: (line: number, column: number) => void
    /** How to colour the output; by default JSON when it is JSON. */
    language?: Language
    badges?: Snippet
    /** The result drawn some other way than as code (no size, no find). */
    rich?: Snippet
    children?: Snippet
  }

  let { output, error, busy, emptyIcon, emptyTitle, emptyDescription, onsample, onswap, onjump, language, badges, rich, children }: Props = $props()

  const shown = $derived<Language>(language ?? displayLanguage(output ?? ''))

  // Find in the output, as in OutputPane.
  let code = $state<CodeView>()
  let finding = $state(false)
  let over = $state(false)
  let pane = $state<HTMLDivElement>()
  const canFind = $derived(output !== null && !rich)
  $effect(() => {
    if (!canFind) finding = false
  })
  findShortcut(
    {
      get pane() {
        return pane
      },
      get over() {
        return over
      },
      get enabled() {
        return canFind
      },
    },
    () => (finding = true),
  )
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="out" bind:this={pane} onpointerenter={() => (over = true)} onpointerleave={() => (over = false)}>
<Pane title="Output">
  {#snippet meta()}
    {#if badges}{@render badges()}{/if}
    {#if output !== null && !rich}<span>{sizeLabel(output)}</span>{/if}
    {#if busy}<Working />{/if}
  {/snippet}
  {#snippet actions()}
    {#if onswap}
      <IconButton icon={ArrowLeftRight} tone="swap" label="Use output as input" shortcut="Alt+S" size="sm" disabled={!output} onclick={onswap} />
    {/if}
    {#if canFind}
      <IconButton icon={Search} label="Find in the output" shortcut="Ctrl+F" size="sm" active={finding} onclick={() => (finding = !finding)} />
    {/if}
    <Button icon={Copy} tone="copy" variant="ghost" size="sm" title="Copy output (Alt+C)" aria-keyshortcuts="Alt+C" disabled={!output} onclick={() => copyText(output ?? '')}>Copy</Button>
  {/snippet}

  {#if error}
    <ErrorBanner {error} {onjump} />
  {/if}
  {#if output !== null && rich}
    <div class="rich">{@render rich()}</div>
  {:else if output !== null}
    {#if finding}<FindBar find={(q, d) => code?.find(q, d) ?? { count: 0, index: 0 }} text={output} onclose={() => (finding = false)} />{/if}
    <div class="stack">
      <div class="code"><CodeView bind:this={code} value={output} label="Output" language={shown} readonly wrap /></div>
      {#if children}{@render children()}{/if}
    </div>
  {:else if !error}
    <EmptyState icon={emptyIcon} title={emptyTitle} description={emptyDescription}>
      {#if onsample}<Button size="sm" onclick={onsample}>Try a sample</Button>{/if}
    </EmptyState>
  {/if}
</Pane>
</div>

<style>
  .out {
    display: contents;
  }
  .rich {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
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
</style>
