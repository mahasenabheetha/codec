<script lang="ts">
  import type { SourceRef } from '../../lib/api/logs'

  // A text with its file and line references as links (decision 99):
  // a click opens the reference in the Source drawer. A reference
  // whose file isn't in the open folder is still a link, shown quieter;
  // the drawer says why.
  interface Props {
    text: string
    refs?: SourceRef[]
    found?: (r: SourceRef) => boolean
    onref: (r: SourceRef) => void
  }

  let { text, refs = [], found = () => true, onref }: Props = $props()

  const pieces = $derived.by(() => {
    const out: { text: string; ref?: SourceRef }[] = []
    let at = 0
    for (const r of refs) {
      if (r.start < at || r.end > text.length) continue
      if (r.start > at) out.push({ text: text.slice(at, r.start) })
      out.push({ text: text.slice(r.start, r.end), ref: r })
      at = r.end
    }
    if (at < text.length) out.push({ text: text.slice(at) })
    return out
  })
</script>

{#each pieces as p, i (i)}{#if p.ref}<button
      type="button"
      class="ref"
      class:missing={!found(p.ref)}
      title="Show {p.ref.path}:{p.ref.line} from the open folder"
      onclick={() => onref(p.ref!)}>{p.text}</button
    >{:else}{p.text}{/if}{/each}

<style>
  .ref {
    display: inline;
    padding: 0;
    font: inherit;
    color: var(--accent);
    text-align: left;
    white-space: inherit;
    overflow-wrap: inherit;
    background: none;
    border: none;
    text-decoration: underline;
    text-decoration-color: color-mix(in oklab, var(--accent) 40%, transparent);
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .ref:hover {
    text-decoration-color: var(--accent);
  }
  .ref.missing {
    color: inherit;
    text-decoration-style: dotted;
    text-decoration-color: var(--fg-2);
  }
</style>
