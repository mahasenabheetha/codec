<script lang="ts">
  // Content already on screen is being worked out again: a dot in the
  // tool's colour and small muted words in a header, never a spinner
  // (design language, "States"). Hidden for the first 250 ms: local work
  // usually finishes sooner, and words that flash read as jank.
  interface Props {
    label?: string
  }
  let { label = 'Working…' }: Props = $props()
</script>

<span class="working" role="status"><span class="pip" aria-hidden="true"></span>{label}</span>

<style>
  .working {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
    opacity: 0;
    animation: appear 1ms 250ms forwards;
  }
  .pip {
    flex: none;
    width: 7px;
    height: 7px;
    background: var(--tool-accent, var(--accent));
    border-radius: 50%;
  }
  @keyframes appear {
    to {
      opacity: 1;
    }
  }
</style>
