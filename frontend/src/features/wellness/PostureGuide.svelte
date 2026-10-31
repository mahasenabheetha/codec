<script lang="ts">
  import Check from '@lucide/svelte/icons/check'

  // The posture break's picture: a figure at a desk, seen from the side,
  // slowly sitting up from a slouch, and three checks lighting up one by
  // one as the break goes. Without motion the figure sits up straight
  // and the checks still follow the time.
  interface Props {
    length: number
    left: number
    /** In the corner card: the figure only. */
    compact?: boolean
  }

  let { length, left, compact = false }: Props = $props()

  const checks = ['Feet flat on the floor', 'Shoulders down and back', 'Screen at eye level']
  const done = $derived(Math.min(checks.length, Math.max(0, Math.floor(((length - left) / Math.max(1, length)) * (checks.length + 0.6)))))
</script>

<div class="posture" class:compact>
  <svg viewBox="0 0 160 160" aria-hidden="true">
    <!-- desk and screen -->
    <line class="desk" x1="96" y1="96" x2="150" y2="96" />
    <line class="desk" x1="140" y1="96" x2="140" y2="148" />
    <rect class="screen" x="118" y="56" width="26" height="20" rx="3" />
    <line class="desk" x1="131" y1="76" x2="131" y2="96" />
    <!-- chair -->
    <line class="chair" x1="40" y1="112" x2="78" y2="112" />
    <line class="chair" x1="42" y1="112" x2="38" y2="64" />
    <line class="chair" x1="58" y1="112" x2="58" y2="148" />
    <!-- legs: thigh on the seat, shin down, foot flat -->
    <line class="body" x1="58" y1="106" x2="92" y2="106" />
    <line class="body" x1="92" y1="106" x2="92" y2="146" />
    <line class="body" x1="92" y1="146" x2="104" y2="146" />
    <!-- upper body pivots at the hip -->
    <g class="upper">
      <line class="body" x1="58" y1="106" x2="58" y2="62" />
      <circle class="head" cx="60" cy="46" r="11" />
      <line class="body" x1="58" y1="70" x2="78" y2="86" />
      <line class="body" x1="78" y1="86" x2="102" y2="90" />
    </g>
    <!-- eye line to the screen -->
    <line class="sight" x1="72" y1="46" x2="118" y2="64" />
  </svg>
  {#if !compact}
    <ul class="checks">
      {#each checks as c, i (c)}
        <li class:done={i < done}><span class="tick"><Check size={12} strokeWidth={3} /></span>{c}</li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .posture {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s-3);
  }
  svg {
    width: 190px;
    height: 170px;
    overflow: visible;
  }
  .compact svg {
    width: 200px;
    height: 200px;
  }
  line {
    stroke-linecap: round;
  }
  .body {
    stroke: var(--rest);
    stroke-width: 5;
  }
  .head {
    fill: none;
    stroke: var(--rest);
    stroke-width: 4;
  }
  .desk,
  .chair {
    stroke: var(--fg-2);
    stroke-width: 3;
  }
  .screen {
    fill: none;
    stroke: var(--fg-2);
    stroke-width: 3;
  }
  .sight {
    stroke: var(--rest);
    stroke-width: 1.5;
    stroke-dasharray: 3 4;
    opacity: 0.7;
  }
  /* From a slouch to sitting tall, then a gentle hold. */
  .upper {
    transform-box: view-box;
    transform-origin: 58px 106px;
    animation: sit-up 6s ease-in-out infinite;
  }
  @keyframes sit-up {
    0%,
    15% {
      transform: rotate(16deg) translateY(4px);
    }
    45%,
    100% {
      transform: rotate(0deg);
    }
  }
  .checks {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--fs-md);
    color: var(--fg-2);
  }
  .checks li {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    transition: color 400ms var(--ease);
  }
  .tick {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    color: transparent;
    border: 2px solid var(--border-strong);
    border-radius: 50%;
    transition:
      background 400ms var(--ease),
      border-color 400ms var(--ease),
      color 400ms var(--ease);
  }
  .checks li.done {
    color: var(--fg-0);
  }
  .done .tick {
    color: var(--bg-0);
    background: var(--rest);
    border-color: var(--rest);
  }
</style>
