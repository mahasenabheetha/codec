<script lang="ts" generics="T extends string">
  import type { Component } from 'svelte'

  // Mutually exclusive choice rendered as a pill group (radio semantics).
  interface Option {
    value: T
    label: string
    icon?: Component
    title?: string
    /** A count after the label, in a status colour when it is one. */
    count?: number
    countTone?: 'neutral' | 'err' | 'warn'
  }

  interface Props {
    options: Option[]
    value: T
    label: string
    /** The choice in a soft fill rather than solid: for a setting (an
     *  interface size, local or UTC) rather than what the view shows. */
    soft?: boolean
    onchange?: (value: T) => void
  }

  let { options, value = $bindable(), label, soft = false, onchange }: Props = $props()

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

<div class="segmented" class:soft role="radiogroup" aria-label={label} tabindex="-1" {onkeydown}>
  {#each options as opt (opt.value)}
    {@const checked = opt.value === value}
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      tabindex={checked ? 0 : -1}
      class:checked
      title={opt.title}
      onclick={() => select(opt.value)}
    >
      {#if opt.icon}<opt.icon size={14} />{/if}
      {opt.label}
      {#if opt.count !== undefined}<span class="count {opt.countTone ?? 'neutral'}">{opt.count}</span>{/if}
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
    /* The buttons' radius plus the padding, so the corners nest. */
    border-radius: calc(var(--r-md) + 2px);
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
    border-radius: var(--r-md);
    transition:
      background var(--dur-fast) var(--ease),
      color var(--dur-fast) var(--ease);
  }
  button:hover:not(.checked) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  /* Solid in the tool's colour: the accent outside a tool and with tool
     colours off. */
  button.checked {
    font-weight: var(--fw-semibold);
    color: var(--accent-fg);
    background: var(--tool-accent, var(--accent));
  }
  .soft button.checked {
    color: var(--fg-0);
    background: var(--tool-accent-soft, var(--accent-soft));
  }
  .count {
    min-width: 16px;
    padding: 0 var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    line-height: 16px;
    text-align: center;
    border-radius: var(--r-sm);
  }
  .count.neutral {
    color: var(--fg-1);
    background: var(--bg-3);
  }
  .count.err {
    color: var(--err);
    background: var(--err-soft);
  }
  .count.warn {
    color: var(--warn);
    background: var(--warn-soft);
  }
</style>
