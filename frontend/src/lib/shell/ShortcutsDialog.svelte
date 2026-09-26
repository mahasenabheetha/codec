<script lang="ts">
  import Dialog from '../components/Dialog.svelte'
  import Kbd from '../components/Kbd.svelte'
  import { layout } from '../stores/layout.svelte'

  const sections: { title: string; items: [string, string][] }[] = [
    {
      title: 'Everywhere',
      items: [
        ['Mod+K', 'Search tools and actions'],
        ['Mod+B', 'Toggle sidebar'],
        ['?', 'Show keyboard shortcuts'],
      ],
    },
    {
      title: 'In a tool',
      items: [
        ['Mod+Enter', 'Run the transform'],
        ['Alt+C', 'Copy output'],
        ['Alt+S', 'Use output as input'],
        ['Escape', 'Clear input and output'],
      ],
    },
    {
      title: 'In an editor',
      items: [
        ['Mod+F', 'Find'],
        ['Mod+Z', 'Undo'],
        ['Mod+Shift+Z', 'Redo'],
      ],
    },
  ]
</script>

<Dialog bind:open={layout.shortcutsOpen} title="Keyboard shortcuts" width="460px">
  {#each sections as s (s.title)}
    <h3>{s.title}</h3>
    <dl>
      {#each s.items as [combo, what] (combo)}
        <dt>{what}</dt>
        <dd><Kbd {combo} /></dd>
      {/each}
    </dl>
  {/each}
</Dialog>

<style>
  h3 {
    margin: var(--s-4) 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  h3:first-child {
    margin-top: 0;
  }
  dl {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: var(--s-2) var(--s-4);
    margin: 0;
  }
  dt {
    color: var(--fg-1);
  }
  dd {
    margin: 0;
  }
</style>
