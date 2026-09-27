<script lang="ts" generics="T extends string">
  // View switcher with underline indicator (tab semantics). Renders the
  // tab list only; the parent shows content for the selected value.
  interface Tab {
    value: T
    label: string
    count?: number
  }

  interface Props {
    tabs: Tab[]
    value: T
    label: string
  }

  let { tabs, value = $bindable(), label }: Props = $props()

  function onkeydown(e: KeyboardEvent) {
    const i = tabs.findIndex((t) => t.value === value)
    let next = -1
    if (e.key === 'ArrowRight') next = (i + 1) % tabs.length
    if (e.key === 'ArrowLeft') next = (i - 1 + tabs.length) % tabs.length
    if (next < 0) return
    e.preventDefault()
    value = tabs[next].value
    ;(e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('button')[next]?.focus()
  }
</script>

<div class="tabs" role="tablist" aria-label={label} tabindex="-1" {onkeydown}>
  {#each tabs as tab (tab.value)}
    {@const selected = tab.value === value}
    <button
      type="button"
      role="tab"
      aria-selected={selected}
      tabindex={selected ? 0 : -1}
      class:selected
      onclick={() => (value = tab.value)}
    >
      {tab.label}
      {#if tab.count !== undefined}<span class="count">{tab.count}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  button {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    height: 32px;
    padding: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-medium);
    color: var(--fg-1);
    background: none;
    border: none;
  }
  button:hover {
    color: var(--fg-0);
  }
  button.selected {
    color: var(--fg-0);
  }
  button.selected::after {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    bottom: -1px;
    height: 2px;
    background: var(--accent);
    border-radius: 2px 2px 0 0;
  }
  .count {
    min-width: 18px;
    padding: 0 5px;
    font-size: var(--fs-xs);
    line-height: 16px;
    text-align: center;
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: var(--r-full);
  }
</style>
