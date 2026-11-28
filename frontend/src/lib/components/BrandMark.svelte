<script lang="ts">
  // The codec mark, "Spectrum keys" (3.11.0): the app icon's geometry (a
  // YAML key over two nested key: value rows) with each key and value in
  // its own tool hue, so it follows the theme. Decorative, so it keeps its
  // colours with Tool colours off.
  let { size = 28 }: { size?: number } = $props()
</script>

<svg class="mark" width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
  <rect class="tile" x="1" y="1" width="62" height="62" rx="14" />
  <g class="bars">
    <path class="indigo" d="M15.5 19.5h15" />
    <path class="pink" d="M24.5 32h9" />
    <path class="teal" d="M24.5 44.5h9" />
    <path class="peach" d="M42.5 32h6" />
    <path class="lime" d="M42.5 44.5h6" />
  </g>
</svg>

<style>
  .mark {
    flex: none;
  }
  .tile {
    fill: var(--bg-2);
    stroke: var(--border-strong);
    stroke-width: 2;
  }
  .bars {
    fill: none;
    stroke-width: 7;
    stroke-linecap: round;
  }
  .indigo {
    stroke: var(--hue-indigo);
  }
  .pink {
    stroke: var(--hue-pink);
  }
  .teal {
    stroke: var(--hue-teal);
  }
  .peach {
    stroke: var(--hue-peach);
  }
  .lime {
    stroke: var(--hue-lime);
  }
</style>
