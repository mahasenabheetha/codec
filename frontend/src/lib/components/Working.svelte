<script lang="ts">
  // Content already on screen is being worked out again: small muted
  // words in a header, never a spinner (design language, "States").
  // Hidden for the first 250 ms: local work usually finishes sooner,
  // and words that flash read as jank.
  interface Props {
    label?: string
  }
  let { label = 'Working…' }: Props = $props()
</script>

<span class="working" role="status">{label}</span>

<style>
  .working {
    font-size: var(--fs-sm);
    color: var(--fg-2);
    white-space: nowrap;
    opacity: 0;
    animation: appear 1ms 250ms forwards;
  }
  @keyframes appear {
    to {
      opacity: 1;
    }
  }
</style>
