<script lang="ts" module>
  import type { EntryField } from '../../lib/api/logs'

  export interface FieldNode {
    name: string // this part of the key
    path: string // the whole key so far
    field?: EntryField
    children: FieldNode[]
  }

  /** Dotted keys as a tree, in the order the fields were written. */
  export function fieldTree(fields: EntryField[]): FieldNode[] {
    const root: FieldNode = { name: '', path: '', children: [] }
    for (const f of fields) {
      let node = root
      const parts = f.key.split('.')
      parts.forEach((part, i) => {
        const path = parts.slice(0, i + 1).join('.')
        let next = node.children.find((c) => c.name === part && !(i === parts.length - 1 && c.field))
        if (!next) {
          next = { name: part, path, children: [] }
          node.children.push(next)
        }
        if (i === parts.length - 1) next.field = f
        node = next
      })
    }
    // A group with one child reads better as one dotted name.
    const fold = (n: FieldNode): FieldNode => {
      n.children = n.children.map(fold)
      if (!n.field && n.children.length === 1) {
        const c = n.children[0]
        return { ...c, name: `${n.name}.${c.name}` }
      }
      return n
    }
    return root.children.map(fold)
  }
</script>

<script lang="ts">
  import ValueActions from './ValueActions.svelte'

  interface Props {
    fields: EntryField[]
  }

  let { fields }: Props = $props()
  const tree = $derived(fieldTree(fields))
</script>

{#snippet nodes(list: FieldNode[])}
  <ul>
    {#each list as n, i (i)}
      <li>
        {#if n.field}
          <div class="leaf">
            <span class="key" title={n.path}>{n.name}</span>
            <span class="val k-{n.field.kind}">{n.field.text}</span>
            <ValueActions key={n.path} value={n.field.text} column />
          </div>
        {/if}
        {#if n.children.length}
          <details open>
            <summary><span class="key group" title={n.path}>{n.name}</span></summary>
            {@render nodes(n.children)}
          </details>
        {/if}
      </li>
    {/each}
  </ul>
{/snippet}

<div class="tree">{@render nodes(tree)}</div>

<style>
  .tree {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  ul :global(ul) {
    padding-left: var(--s-4);
    border-left: 1px solid var(--border);
    margin-left: 5px;
  }
  .leaf {
    display: grid;
    grid-template-columns: minmax(80px, max-content) minmax(0, 1fr) auto;
    gap: var(--s-3);
    align-items: baseline;
    padding: 1px var(--s-1);
    border-radius: var(--r-sm);
  }
  .leaf:hover {
    background: var(--bg-2);
  }
  .key {
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--syn-key);
  }
  .group {
    color: var(--fg-1);
  }
  summary {
    cursor: pointer;
    padding: 1px var(--s-1);
  }
  .val {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: var(--fg-0);
  }
  .k-number {
    color: var(--syn-number);
  }
  .k-bool,
  .k-null {
    color: var(--syn-bool);
  }
  .k-string {
    color: var(--syn-string);
  }
</style>
