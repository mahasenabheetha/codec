<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import type { TransformResponse } from '../../lib/api/transform'
  import TextOutput from './TextOutput.svelte'
  import type { TransformState } from './transform.svelte'

  // The output pane of a transform tool (Base64, Smart paste, JWT…):
  // TextOutput, fed from the tool's session. One implementation, so the
  // two never drift apart again (3.10.1).
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

  let { session, emptyIcon, emptyTitle, emptyDescription, onsample, onswap, onjump, badges, view }: Props = $props()
</script>

{#snippet drawn()}
  {#if view && session.result}{@render view(session.result)}{/if}
{/snippet}

<TextOutput
  output={session.result ? session.output : null}
  error={session.error}
  busy={session.busy}
  {emptyIcon}
  {emptyTitle}
  {emptyDescription}
  {onsample}
  {onswap}
  {onjump}
  {badges}
  rich={view ? drawn : undefined}
/>
