<script lang="ts">
  import type { RegexNode } from '../../lib/api/regex'
  import ExplainTree from './ExplainTree.svelte'

  // The explanation, part by part. Hovering (or focusing) a part reports
  // it, so the pattern and the matches can light it up.
  interface Props {
    nodes: RegexNode[]
    hovered: RegexNode | null
    onhover: (n: RegexNode | null) => void
    depth?: number
  }

  let { nodes, hovered, onhover, depth = 0 }: Props = $props()
</script>

<ul class="tree" class:root={depth === 0} role={depth === 0 ? 'tree' : 'group'}>
  {#each nodes as n, i (i + ':' + n.start)}
    <li role="treeitem" aria-selected={hovered === n}>
      <div
        class="row k-{n.kind}"
        class:on={hovered === n}
        tabindex="0"
        role="button"
        onmouseenter={() => onhover(n)}
        onmouseleave={() => onhover(null)}
        onfocus={() => onhover(n)}
        onblur={() => onhover(null)}
      >
        <code>{n.text}</code>
        <span class="desc">{n.desc}</span>
      </div>
      {#if n.children?.length}
        <ExplainTree nodes={n.children} {hovered} {onhover} depth={depth + 1} />
      {/if}
    </li>
  {/each}
</ul>

<style>
  .tree {
    margin: 0;
    padding: 0 0 0 var(--s-4);
    list-style: none;
    border-left: 1px solid var(--border);
  }
  .tree.root {
    padding: 0;
    border-left: none;
  }
  .row {
    display: flex;
    align-items: baseline;
    gap: var(--s-3);
    padding: 3px var(--s-2);
    border-radius: var(--r-sm);
    cursor: default;
  }
  .row:hover,
  .row.on {
    background: var(--bg-3);
  }
  code {
    flex: none;
    max-width: 45%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 0 4px;
    font-size: var(--fs-sm);
    color: var(--syn-string);
    background: var(--bg-0);
    border-radius: var(--r-sm);
  }
  .k-group > code {
    color: var(--syn-key);
  }
  .k-quantifier > code,
  .k-alternation > code {
    color: var(--syn-number);
  }
  .k-anchor > code,
  .k-flags > code {
    color: var(--syn-bool);
  }
  .k-class > code,
  .k-escape > code,
  .k-dot > code {
    color: var(--syn-anchor);
  }
  .k-error > code,
  .k-error .desc {
    color: var(--err);
  }
  .desc {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
</style>
