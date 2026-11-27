<script lang="ts">
  // The codec mark (Spectrum, 3.11.0): three bars in three tool hues on a
  // tile. Decorative, so it keeps its colours with Tool colours off.
  let { size = 28 }: { size?: number } = $props()
</script>

<svg class="mark" width={size} height={size} viewBox="0 0 28 28" aria-hidden="true">
  <rect class="tile" x="0.5" y="0.5" width="27" height="27" rx="7.5" />
  <rect class="bar indigo" x="7" y="5" width="10" height="4" rx="2" />
  <rect class="bar pink" x="9" y="12" width="12" height="4" rx="2" />
  <rect class="bar teal" x="11" y="19" width="9" height="4" rx="2" />
</svg>

<style>
  .mark {
    flex: none;
  }
  .tile {
    fill: var(--bg-3);
    stroke: var(--border-strong);
  }
  .indigo {
    fill: var(--hue-indigo);
  }
  .pink {
    fill: var(--hue-pink);
  }
  .teal {
    fill: var(--hue-teal);
  }
</style>
