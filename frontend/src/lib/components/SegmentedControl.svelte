<script lang="ts" generics="T extends string">
  import type { Component } from 'svelte'

  // Mutually exclusive choice rendered as a pill group (radio semantics).
  interface Option {
    value: T
    label: string
    icon?: Component
  }

  interface Props {
    options: Option[]
    value: T
    label: string
    onchange?: (value: T) => void
  }

  let { options, value = $bindable(), label, onchange }: Props = $props()

  function select(v: T) {
    if (v === value) return
    value = v
    onchange?.(v)
  }

  // Arrow keys move the selection, like native radio groups.
  function onkeydown(e: KeyboardEvent) {
    const i = options.findIndex((o) => o.value === value)
    let next = -1
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = (i + 1) % options.length
    if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = (i - 1 + options.length) % options.length
    if (next < 0) return
    e.preventDefault()
    select(options[next].value)
    const group = e.currentTarget as HTMLElement
    group.querySelectorAll<HTMLButtonElement>('button')[next]?.focus()
  }
</script>

<div class="segmented" role="radiogroup" aria-label={label} tabindex="-1" {onkeydown}>
  {#each options as opt (opt.value)}
    {@const checked = opt.value === value}
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      tabindex={checked ? 0 : -1}
      class:checked
      onclick={() => select(opt.value)}
    >
      {#if opt.icon}<opt.icon size={14} strokeWidth={1.75} />{/if}
      {opt.label}
    </button>
  {/each}
</div>

<style>
  .segmented {
    display: inline-flex;
    padding: 2px;
    gap: 2px;
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  button {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    height: calc(var(--control-h) - 6px);
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--fg-1);
    background: transparent;
    border: none;
    border-radius: var(--r-sm);
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  button:hover:not(.checked) {
    color: var(--fg-0);
  }
  button.checked {
    color: var(--fg-0);
    background: var(--bg-3);
    box-shadow: 0 0 0 1px var(--border-strong);
  }
</style>
