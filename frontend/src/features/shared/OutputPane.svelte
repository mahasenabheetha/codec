<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import Copy from '@lucide/svelte/icons/copy'
  import Search from '@lucide/svelte/icons/search'
  import CodeView from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import type { TransformResponse } from '../../lib/api/transform'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from './Pane.svelte'
  import ErrorBanner from './ErrorBanner.svelte'
  import FindBar from './FindBar.svelte'
  import { findShortcut } from './find.svelte'
  import { displayLanguage, sizeLabel } from './detect'
  import type { TransformState } from './transform.svelte'

  interface Props {
    session: TransformState
    emptyIcon: Component
    emptyTitle: string
    emptyDescription: string
    onsample?: () => void
    onswap?: () => void
    onjump?: (line: number, column: number) => void
    badges?: Snippet
    /** Custom rendering of a result; defaults to a read-only code view. */
    view?: Snippet<[TransformResponse]>
  }

  let {
    session,
    emptyIcon,
    emptyTitle,
    emptyDescription,
    onsample,
    onswap,
    onjump,
    badges,
    view,
  }: Props = $props()

  const language = $derived(displayLanguage(session.output))

  // Find in the output: the button, or Ctrl+F while the pointer is over
  // the output or focus is inside it (elsewhere Ctrl+F stays the
  // browser's, and a code editor's own).
  let code = $state<CodeView>()
  let finding = $state(false)
  let over = $state(false)
  let pane = $state<HTMLDivElement>()
  const canFind = $derived(!!session.result && !view)
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
    {#if session.result && !view}<span>{sizeLabel(session.output)}</span>{/if}
    {#if session.busy}<span class="busy" aria-label="Working"></span>{/if}
  {/snippet}
  {#snippet actions()}
    {#if onswap}
      <IconButton
        icon={ArrowLeftRight} tone="swap"
        label="Use output as input"
        shortcut="Alt+S"
        size="sm"
        disabled={!session.output}
        onclick={onswap}
      />
    {/if}
    {#if canFind}
      <IconButton icon={Search} label="Find in the output" shortcut="Ctrl+F" size="sm" active={finding} onclick={() => (finding = !finding)} />
    {/if}
    <Button
      icon={Copy}
      tone="copy"
      variant="ghost"
      size="sm"
      title="Copy output (Alt+C)"
      aria-keyshortcuts="Alt+C"
      disabled={!session.output}
      onclick={() => copyText(session.output)}>Copy</Button
    >
  {/snippet}

  {#if session.error}
    <ErrorBanner error={session.error} {onjump} />
  {/if}

  {#if session.result}
    {#if view}
      <div class="rich">{@render view(session.result)}</div>
    {:else}
      {#if finding}<FindBar find={(q, d) => code?.find(q, d) ?? { count: 0, index: 0 }} text={session.output} onclose={() => (finding = false)} />{/if}
      <CodeView bind:this={code} value={session.output} label="Output" {language} readonly wrap />
    {/if}
  {:else if !session.error}
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
  .busy {
    width: 12px;
    height: 12px;
    border: 2px solid var(--bg-4);
    border-top-color: var(--accent);
    border-radius: 50%;
    /* Hidden for the first 250ms: local transforms usually finish
       sooner, and a flashing spinner reads as jank. */
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
