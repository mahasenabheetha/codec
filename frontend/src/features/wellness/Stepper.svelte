<script lang="ts">
  import Minus from '@lucide/svelte/icons/minus'
  import Plus from '@lucide/svelte/icons/plus'
  import IconButton from '../../lib/components/IconButton.svelte'

  // A number with − and + beside it, kept within min and max.
  interface Props {
    value: number
    label: string
    unit?: string
    min: number
    max: number
    step?: number
  }

  let { value = $bindable(), label, unit = '', min, max, step = 1 }: Props = $props()

  const clamp = (n: number) => Math.min(max, Math.max(min, Math.round(n)))
</script>

<div class="stepper">
  <span class="label">{label}</span>
  <div class="field">
    <IconButton icon={Minus} label="Less {label.toLowerCase()}" size="sm" disabled={value <= min} onclick={() => (value = clamp(value - step))} />
    <input
      type="number"
      {min}
      {max}
      aria-label={label}
      value={value}
      onchange={(e) => (value = clamp(Number((e.currentTarget as HTMLInputElement).value) || min))}
    />
    <IconButton icon={Plus} label="More {label.toLowerCase()}" size="sm" disabled={value >= max} onclick={() => (value = clamp(value + step))} />
    {#if unit}<span class="unit">{unit}</span>{/if}
  </div>
</div>

<style>
  .stepper {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .field {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  input {
    width: 48px;
    height: var(--control-h-sm);
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    text-align: center;
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    appearance: textfield;
  }
  input::-webkit-inner-spin-button {
    appearance: none;
  }
  input:focus {
    outline: none;
    border-color: var(--tool-accent, var(--accent));
  }
  .unit {
    margin-left: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>
