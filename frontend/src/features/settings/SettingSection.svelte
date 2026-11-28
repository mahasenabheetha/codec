<script lang="ts">
  import type { Snippet } from 'svelte'
  import Tile from '../../lib/components/Tile.svelte'
  import { hueVar } from '../../lib/tools'
  import { sectionLook } from './sections'

  // A section of the page: its heading on a solid tile in the section's
  // colour, then its rows in a card. `lead` is a card of its own above
  // them (the themes).
  interface Props {
    id: string
    title: string
    description?: string
    lead?: Snippet
    children: Snippet
  }

  let { id, title, description, lead, children }: Props = $props()
  const look = $derived(sectionLook(id))
</script>

<section id="settings-{id}" class:hued={!!look.hue} style:--tool={look.hue ? hueVar(look.hue) : undefined} aria-labelledby="settings-{id}-title">
  <div class="head">
    <Tile icon={look.icon} size="lg" solid />
    <div>
      <h2 id="settings-{id}-title" tabindex="-1">{title}</h2>
      {#if description}<p>{description}</p>{/if}
    </div>
  </div>
  {#if lead}<div class="card">{@render lead()}</div>{/if}
  <div class="card">{@render children()}</div>
</section>

<style>
  section {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    margin-top: var(--s-8);
    scroll-margin-top: var(--s-4);
  }
  section:first-child {
    margin-top: 0;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-xl);
    font-weight: var(--fw-bold);
    color: var(--fg-0);
  }
  p {
    margin: 2px 0 0;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  /* What is in it (rows, notes, a form) is divided by lines and padded
     here, so each keeps its own top border and no side padding. */
  .card {
    padding: 0 var(--s-4);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .card > :global(:first-child) {
    border-top: none;
  }
</style>
