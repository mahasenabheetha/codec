<script lang="ts">
  // On/off switch with a visible label (switch semantics).
  interface Props {
    checked: boolean
    label: string
    disabled?: boolean
    title?: string
    /** Accessible name when the visible label only states on/off. */
    name?: string
    onchange?: (checked: boolean) => void
  }

  let { checked = $bindable(), label, disabled = false, title, name, onchange }: Props = $props()

  function toggle() {
    checked = !checked
    onchange?.(checked)
  }
</script>

<button type="button" role="switch" aria-checked={checked} aria-label={name} class="toggle" {disabled} {title} onclick={toggle}>
  <span class="track" class:on={checked}><span class="thumb"></span></span>
  <span class="label">{label}</span>
</button>

<style>
  .toggle {
    flex: none;
    display: inline-flex;
    white-space: nowrap;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-md);
  }
  .toggle:hover:not(:disabled) {
    color: var(--fg-0);
  }
  .toggle:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .track {
    flex: none;
    position: relative;
    width: 30px;
    height: 18px;
    background: var(--bg-4);
    border-radius: var(--r-full);
    transition: background var(--dur) var(--ease);
  }
  .track.on {
    background: var(--tool-accent, var(--accent));
  }
  .thumb {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 12px;
    height: 12px;
    background: var(--fg-1);
    border-radius: 50%;
    transition: transform var(--dur) var(--ease);
  }
  .track.on .thumb {
    transform: translateX(12px);
    background: var(--accent-fg);
  }
</style>
