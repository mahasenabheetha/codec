<script lang="ts">
  // A placeholder block shown while content loads: the layout stays put
  // and nothing spins (design/design-language.md). Screen readers hear the label via
  // aria-busy on the container; the block itself is decorative.
  interface Props {
    width?: string
    height?: string
    /** Several lines of varying width, like text. */
    lines?: number
  }

  let { width = '100%', height = '12px', lines = 0 }: Props = $props()
  const widths = [72, 58, 84, 46, 66, 52, 78]
</script>

{#if lines > 0}
  <div class="lines" aria-hidden="true">
    {#each { length: lines } as _, i (i)}
      <span class="block" style:width="{widths[i % widths.length]}%" style:height></span>
    {/each}
  </div>
{:else}
  <span class="block" style:width style:height aria-hidden="true"></span>
{/if}

<style>
  .lines {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .block {
    display: block;
    border-radius: var(--r-sm);
    background: var(--bg-3);
    animation: pulse 1.2s var(--ease) infinite alternate;
  }
  @keyframes pulse {
    to {
      opacity: 0.4;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .block {
      animation: none;
    }
  }
</style>
