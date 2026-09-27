<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    id: string
    title: string
    description?: string
    children: Snippet
  }

  let { id, title, description, children }: Props = $props()
</script>

<section id="settings-{id}" aria-labelledby="settings-{id}-title">
  <h2 id="settings-{id}-title" tabindex="-1">{title}</h2>
  {#if description}<p>{description}</p>{/if}
  {@render children()}
</section>

<style>
  section {
    margin-top: var(--s-8);
    scroll-margin-top: var(--s-4);
  }
  section:first-child {
    margin-top: 0;
  }
  h2 {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-1);
  }
  p {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
</style>
