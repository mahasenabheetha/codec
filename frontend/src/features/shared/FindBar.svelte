<script lang="ts">
  import { tick } from 'svelte'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ChevronUp from '@lucide/svelte/icons/chevron-up'
  import X from '@lucide/svelte/icons/x'
  import IconButton from '../../lib/components/IconButton.svelte'

  // Find in a result: type and every match is marked, Enter goes to the
  // next, Shift+Enter to the previous, Esc closes. `find` is the code
  // view's (CodeView.find); the text searched is its current text.
  interface Props {
    find: (q: string, dir: 0 | 1 | -1 | 'mark') => { count: number; index: number }
    onclose: () => void
    /** Changes whenever the text searched changes, to search it again. */
    text: string
    /** What is searched, for the placeholder: "output" or "input". */
    what?: string
    /** Text that changes as you type (an input): marks and counts follow
     *  it without moving the cursor. Otherwise a new text starts again
     *  from its first match. */
    stay?: boolean
  }

  let { find, onclose, text, what = 'output', stay = false }: Props = $props()

  let q = $state('')
  let found = $state({ count: 0, index: 0 })
  let input = $state<HTMLInputElement>()

  $effect(() => {
    void tick().then(() => input?.focus())
    return () => find('', 0) // clear the marks when the bar goes
  })

  // A new query starts from the first match; new text under the same
  // query does too, unless it is being typed (stay).
  let lastQuery = ''
  $effect(() => {
    void text
    const query = q
    const fresh = query !== lastQuery
    lastQuery = query
    found = find(query, stay && !fresh ? 'mark' : 0)
  })

  function step(dir: 1 | -1) {
    if (q) found = find(q, dir)
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      step(e.shiftKey ? -1 : 1)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      onclose()
    }
  }
</script>

<div class="find" role="search">
  <input bind:this={input} bind:value={q} {onkeydown} placeholder="Find in the {what}" aria-label="Find in the {what}" spellcheck="false" />
  <span class="count" aria-live="polite" class:none={!!q && !found.count}>
    {#if !q}{:else if !found.count}no matches{:else}{found.index || 1} of {found.count >= 10_000 ? '10,000+' : found.count.toLocaleString('en-US')}{/if}
  </span>
  <IconButton icon={ChevronUp} label="Previous match" shortcut="Shift+Enter" size="sm" disabled={!found.count} onclick={() => step(-1)} />
  <IconButton icon={ChevronDown} label="Next match" shortcut="Enter" size="sm" disabled={!found.count} onclick={() => step(1)} />
  <IconButton icon={X} label="Close find" shortcut="Escape" size="sm" onclick={onclose} />
</div>

<style>
  /* A compact bar on the right, like an editor's find widget. */
  .find {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2);
    background: var(--bg-1);
    border-bottom: 1px solid var(--border);
  }
  input {
    flex: 0 1 260px;
    min-width: 0;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
  }
  .count {
    min-width: 72px;
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    text-align: right;
    color: var(--fg-2);
  }
  .count.none {
    color: var(--warn);
  }
</style>
