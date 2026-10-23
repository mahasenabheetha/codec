<script lang="ts">
  import Highlighter from '@lucide/svelte/icons/highlighter'
  import Button from '../../lib/components/Button.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import { ruleTones } from './format'
  import { useViewer } from './viewer.svelte'

  // Up to four highlight rules: queries whose entries get a colour on
  // their edge and a mark beside the scrollbar, without hiding the
  // rest. Kept for this tab only, in memory.
  const viewer = useViewer()

  let draft = $state(['', '', '', ''])
  // The rules in use, back in the boxes whenever they change.
  $effect(() => {
    const h = viewer.highlights
    draft = [0, 1, 2, 3].map((k) => h[k] ?? '')
  })

  const active = $derived(viewer.highlights.length)
  // The server's error for a rule, matched to its box (empty boxes are dropped before sending).
  const errorFor = $derived.by(() => {
    const out = ['', '', '', '']
    let k = 0
    draft.forEach((d, j) => {
      if (d.trim() && viewer.highlights[k] === d.trim()) out[j] = viewer.highlightErrors[k++] ?? ''
    })
    return out
  })

  function apply() {
    viewer.setHighlights(draft)
  }
</script>

<Popover side="bottom" align="end">
  {#snippet trigger(props)}
    <Button {...props} size="sm" variant="ghost" icon={Highlighter}>Highlight{active ? ` (${active})` : ''}</Button>
  {/snippet}
  <form
    class="rules"
    onsubmit={(e) => {
      e.preventDefault()
      apply()
    }}
  >
    <p class="muted">Mark entries in colour without hiding the others. Each rule is a search, written like the query.</p>
    {#each draft as _, k (k)}
      <label class="rule">
        <i style:background={ruleTones[k]}></i>
        <input bind:value={draft[k]} placeholder={['timeout', 'level>=warn', 'source:*db*', 'trace.id:…'][k]} spellcheck="false" autocomplete="off" aria-label="Highlight rule {k + 1}" />
      </label>
      {#if errorFor[k]}<p class="err">{errorFor[k]}</p>{/if}
    {/each}
    <div class="actions">
      {#if active}<Button size="sm" variant="ghost" onclick={() => viewer.setHighlights([])}>Clear all</Button>{/if}
      <Button size="sm" variant="primary" type="submit">Apply</Button>
    </div>
  </form>
</Popover>

<style>
  .rules {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    width: 320px;
    padding: var(--s-3);
  }
  .rules p {
    margin: 0;
    font-size: var(--fs-xs);
  }
  .muted {
    color: var(--fg-2);
  }
  .err {
    color: var(--err);
  }
  .rule {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .rule i {
    flex: none;
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }
  .rule input {
    flex: 1;
    min-width: 0;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
