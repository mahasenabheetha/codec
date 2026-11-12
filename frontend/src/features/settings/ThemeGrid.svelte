<script lang="ts">
  import Monitor from '@lucide/svelte/icons/monitor'
  import { theme, themes, type Theme, type ThemePref } from '../../lib/stores/theme.svelte'

  // The themes as a grid of swatches (3.10.2), in place of a menu of ten
  // names: each card is the theme's own ground, text, accent and one tool
  // colour, so you pick by looking. A radio group: arrow keys move and
  // choose, as radio buttons do.
  const options: { value: ThemePref; label: string; note: string; swatch?: [string, string, string, string] }[] = [
    { value: 'system', label: 'System', note: "follows your operating system's light or dark" },
    ...themes.map((t) => ({ value: t.value as ThemePref, label: t.label, note: t.note, swatch: t.swatch })),
  ]

  function pick(v: ThemePref) {
    if (v === 'system') theme.pref = 'system'
    else theme.use(v as Theme)
  }

  let group = $state<HTMLDivElement>()
  function onkeydown(e: KeyboardEvent) {
    const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const at = options.findIndex((o) => o.value === theme.pref)
    const next = options[(at + step + options.length) % options.length]
    pick(next.value)
    group?.querySelector<HTMLElement>(`[data-theme-option="${next.value}"]`)?.focus()
  }
</script>

<!-- svelte-ignore a11y_interactive_supports_focus -->
<div class="grid" role="radiogroup" aria-label="Theme" bind:this={group} {onkeydown}>
  {#each options as o (o.value)}
    {@const on = theme.pref === o.value}
    <button type="button" role="radio" aria-checked={on} tabindex={on ? 0 : -1} class="card" class:on data-theme-option={o.value} title={o.note} onclick={() => pick(o.value)}>
      {#if o.swatch}
        <span class="swatch" style:background={o.swatch[0]} style:color={o.swatch[1]} aria-hidden="true">
          <span class="aa">Aa</span>
          <span class="dots"><i style:background={o.swatch[2]}></i><i style:background={o.swatch[3]}></i></span>
        </span>
      {:else}
        <span class="swatch system" aria-hidden="true"><Monitor size={18} strokeWidth={1.75} /></span>
      {/if}
      <span class="name">{o.label}</span>
      <span class="note">{o.note}</span>
    </button>
  {/each}
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(128px, 1fr));
    gap: var(--s-2);
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-2);
    text-align: left;
    color: var(--fg-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    transition:
      border-color var(--dur) var(--ease),
      background var(--dur) var(--ease);
  }
  .card:hover {
    border-color: var(--border-strong);
    background: var(--bg-2);
  }
  /* The theme in use: a firm frame and a check of colour, not colour alone
     (aria-checked says it too). */
  .card.on {
    color: var(--fg-0);
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }
  .swatch {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 44px;
    margin-bottom: var(--s-1);
    padding: 0 var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .swatch.system {
    justify-content: center;
    color: var(--fg-1);
    background: linear-gradient(105deg, var(--bg-0) 50%, var(--bg-3) 50%);
  }
  .aa {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .dots {
    display: flex;
    gap: var(--s-1);
  }
  .dots i {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }
  .name {
    font-size: var(--fs-md);
    font-weight: var(--fw-medium);
  }
  .note {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
