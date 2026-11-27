<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLAttributes } from 'svelte/elements'

  // A framed block on the panel: a home card, a setting's group, a result
  // that stands apart. `hue` gives it the hue edge: a border in the
  // tool's colour on a ground washed with it (the selected one, a
  // callout); with `status`, in that status's colour instead. `lift`
  // raises it a little, for one that moves under the pointer.
  interface Props extends HTMLAttributes<HTMLElement> {
    tag?: 'div' | 'section' | 'article' | 'li'
    hue?: boolean
    status?: 'ok' | 'warn' | 'err' | 'info'
    lift?: boolean
    children: Snippet
  }

  let { tag = 'div', hue = false, status, lift = false, children, class: cls = '', ...rest }: Props = $props()
</script>

<svelte:element this={tag} class="card {status ?? ''} {cls}" class:hue class:status={!!status} class:lift {...rest}>
  {@render children()}
</svelte:element>

<style>
  .card {
    min-width: 0;
    padding: var(--s-4);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .lift {
    box-shadow: var(--shadow-raise);
  }
  .hue {
    background: var(--tool-wash, var(--bg-1));
    border-color: var(--tool-line, var(--border));
  }

  .ok {
    --st: var(--ok);
  }
  .warn {
    --st: var(--warn);
  }
  .err {
    --st: var(--err);
  }
  .info {
    --st: var(--info);
  }
  .hue.status {
    background: color-mix(in srgb, var(--st) var(--wash), var(--bg-1));
    border-color: color-mix(in srgb, var(--st) 45%, var(--border));
  }
</style>
