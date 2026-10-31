<script lang="ts">
  import type { Snippet } from 'svelte'

  // The focus timer's dial: a soft track, faint minute ticks, and an arc
  // that fills clockwise in the phase's colour, ending in a small knob.
  interface Props {
    /** 0 to 1. */
    progress: number
    /** A CSS colour, e.g. var(--rest). */
    color: string
    running?: boolean
    children?: Snippet
  }

  let { progress, color, running = false, children }: Props = $props()

  const R = 88
  const C = 2 * Math.PI * R
  const p = $derived(Math.min(1, Math.max(0, progress)))
  const angle = $derived(p * 2 * Math.PI - Math.PI / 2)
  const ticks = Array.from({ length: 60 }, (_, i) => i)
</script>

<div class="ring" class:running style:--ring={color}>
  <svg viewBox="0 0 200 200" aria-hidden="true">
    {#each ticks as i (i)}
      {@const a = (i / 60) * 2 * Math.PI}
      {@const long = i % 5 === 0}
      <line
        class="tick"
        class:long
        x1={100 + Math.sin(a) * 97}
        y1={100 - Math.cos(a) * 97}
        x2={100 + Math.sin(a) * (long ? 93 : 95)}
        y2={100 - Math.cos(a) * (long ? 93 : 95)}
      />
    {/each}
    <circle class="track" cx="100" cy="100" r={R} />
    <circle
      class="arc"
      cx="100"
      cy="100"
      r={R}
      stroke-dasharray={C}
      stroke-dashoffset={C * (1 - p)}
      transform="rotate(-90 100 100)"
    />
    {#if p > 0}
      <circle class="knob" cx={100 + Math.cos(angle) * R} cy={100 + Math.sin(angle) * R} r="6" />
    {/if}
  </svg>
  <div class="center">{@render children?.()}</div>
</div>

<style>
  .ring {
    position: relative;
    width: min(300px, 72vw);
    aspect-ratio: 1;
  }
  svg {
    width: 100%;
    height: 100%;
    overflow: visible;
  }
  .tick {
    stroke: var(--fg-2);
    stroke-width: 0.6;
    opacity: 0.35;
  }
  .tick.long {
    stroke-width: 1;
    opacity: 0.6;
  }
  .track {
    fill: none;
    stroke: color-mix(in oklab, var(--ring) 16%, var(--bg-3));
    stroke-width: 9;
  }
  .arc {
    fill: none;
    stroke: var(--ring);
    stroke-width: 9;
    stroke-linecap: round;
    transition:
      stroke-dashoffset 300ms linear,
      stroke 300ms var(--ease);
  }
  .knob {
    fill: var(--bg-1);
    stroke: var(--ring);
    stroke-width: 3;
    transition: stroke 300ms var(--ease);
  }
  /* A soft glow while it runs. */
  .running .arc {
    filter: drop-shadow(0 0 5px color-mix(in oklab, var(--ring) 45%, transparent));
  }
  .center {
    position: absolute;
    inset: 16%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--s-1);
    text-align: center;
  }
</style>
