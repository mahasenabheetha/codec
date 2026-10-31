<script lang="ts">
  // The water break's picture: a glass that fills as the break goes, a
  // slow wave on top and a few bubbles rising. Without motion it is the
  // glass filling, still.
  let { progress }: { progress: number } = $props()

  // The water's top, from the glass's foot (y 150) to near its rim (y 40).
  const top = $derived(150 - Math.min(1, Math.max(0.12, progress)) * 110)
</script>

<div class="water" aria-hidden="true">
  <svg viewBox="0 0 120 170">
    <defs>
      <clipPath id="codec-glass">
        <path d="M24 26 L96 26 L86 152 Q85 158 79 158 L41 158 Q35 158 34 152 Z" />
      </clipPath>
    </defs>
    <g clip-path="url(#codec-glass)">
      <g class="fill" style:transform="translateY({top - 40}px)">
        <path class="wave" d="M-60 40 Q-45 34 -30 40 T0 40 T30 40 T60 40 T90 40 T120 40 T150 40 T180 40 V200 H-60 Z" />
        <circle class="bubble" cx="48" cy="120" r="2.5" />
        <circle class="bubble b2" cx="66" cy="132" r="2" />
        <circle class="bubble b3" cx="58" cy="110" r="1.6" />
      </g>
    </g>
    <path class="glass" d="M24 26 L96 26 L86 152 Q85 158 79 158 L41 158 Q35 158 34 152 Z" />
    <path class="shine" d="M33 36 L40 140" />
  </svg>
</div>

<style>
  .water {
    display: grid;
    place-items: center;
    width: 200px;
    height: 200px;
  }
  svg {
    width: 140px;
    height: 190px;
    overflow: visible;
  }
  .glass {
    fill: none;
    stroke: var(--rest);
    stroke-width: 3.5;
    stroke-linejoin: round;
  }
  .shine {
    stroke: color-mix(in oklab, var(--fg-0) 35%, transparent);
    stroke-width: 3;
    stroke-linecap: round;
  }
  .fill {
    transition: transform 900ms ease-out;
  }
  .wave {
    fill: color-mix(in oklab, var(--rest) 55%, transparent);
    animation: wave 3s linear infinite;
  }
  @keyframes wave {
    to {
      transform: translateX(60px);
    }
  }
  .bubble {
    fill: color-mix(in oklab, var(--fg-0) 55%, transparent);
    animation: rise 3.2s ease-in infinite;
  }
  .b2 {
    animation-delay: -1.1s;
  }
  .b3 {
    animation-delay: -2.2s;
  }
  @keyframes rise {
    from {
      transform: translateY(0);
      opacity: 0.9;
    }
    to {
      transform: translateY(-70px);
      opacity: 0;
    }
  }
</style>
