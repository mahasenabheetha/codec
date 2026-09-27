<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity'
  import ListTree from '@lucide/svelte/icons/list-tree'
  import TreeView, { type Row } from '../../lib/components/TreeView.svelte'
  import type { FileSession, SymbolRef } from './session.svelte'

  // The file's outline, synced with the cursor both ways: the symbol
  // under the cursor is highlighted and revealed; clicking one jumps.
  interface Props {
    session: FileSession
    onjump: (ref: SymbolRef) => void
  }

  let { session, onjump }: Props = $props()

  // Collapsed rather than expanded, so a fresh outline starts open two
  // levels deep and new symbols appear expanded as the user types.
  const collapsed = new SvelteSet<string>()

  const byId = $derived(new Map(session.symbols.map((r) => [r.id, r])))
  const multiDoc = $derived((session.analysis?.docs.length ?? 0) > 1)

  const depthOf = (id: string) => id.split('/').length - 2
  const parentId = (id: string) => id.slice(0, id.lastIndexOf('/'))

  // Big outlines (many documents) start collapsed: rendering thousands
  // of rows would make every analysis, and so typing, sluggish.
  const openByDefault = $derived(session.symbols.length <= 400)

  function isOpen(ref: SymbolRef): boolean {
    if (collapsed.has(ref.id)) return false
    return (openByDefault && depthOf(ref.id) < 1) || revealed.has(ref.id) || expandedByUser.has(ref.id)
  }

  // Ancestors of the current symbol stay open so it is always visible.
  // Derived from a string, so rows are rebuilt only when the cursor
  // moves to another parent, not on every keystroke.
  const currentParent = $derived(session.current ? parentId(session.current.id) : '')
  const revealed = $derived.by(() => {
    const s = new Set<string>()
    for (let id = currentParent; id.includes('/'); id = parentId(id)) s.add(id)
    return s
  })
  const expandedByUser = new SvelteSet<string>()

  const rows = $derived.by<Row<SymbolRef>[]>(() => {
    const out: Row<SymbolRef>[] = []
    for (const ref of session.symbols) {
      // Visible if every ancestor is open.
      let visible = true
      for (let id = parentId(ref.id); id.includes('/'); id = parentId(id)) {
        const p = byId.get(id)
        if (p && !isOpen(p)) {
          visible = false
          break
        }
      }
      if (!visible) continue
      const expandable = !!ref.sym.children?.length
      out.push({ id: ref.id, depth: depthOf(ref.id), expandable, expanded: expandable && isOpen(ref), data: ref })
    }
    return out
  })

  function toggle(id: string) {
    const ref = byId.get(id)
    if (!ref) return
    if (isOpen(ref)) {
      collapsed.add(id)
      expandedByUser.delete(id)
    } else {
      collapsed.delete(id)
      expandedByUser.add(id)
    }
  }

  // The first visible row of each document gets its number, so a
  // multi-document file reads as sections.
  const docOf = (id: string) => Number(id.split('/')[0])
  const firstOfDoc = $derived.by(() => {
    const seen = new Set<number>()
    const ids = new Set<string>()
    for (const r of rows) if (!seen.has(docOf(r.id))) (seen.add(docOf(r.id)), ids.add(r.id))
    return ids
  })
</script>

<div class="outline">
  {#if !session.isYAML}
    <p class="hint">Outline is available for YAML files.</p>
  {:else if session.symbols.length === 0}
    <p class="hint"><ListTree size={14} strokeWidth={1.75} /> Nothing to outline yet.</p>
  {:else}
    {#if session.analysis?.outlineTrimmed}
      <p class="hint trimmed">Shortened: this file is too big to outline every key.</p>
    {/if}
    <TreeView
      {rows}
      label="Outline"
      active={session.current?.id ?? null}
      toggleOnClick={false}
      ontoggle={toggle}
      onopen={onjump}
    >
      {#snippet row(ref, r)}
        {#if multiDoc && firstOfDoc.has(r.id)}
          <span class="doc" title="Document {docOf(r.id) + 1}">{docOf(r.id) + 1}</span>
        {/if}
        <span class="name kind-{ref.sym.kind}">{ref.sym.name}</span>
        {#if ref.sym.detail}<span class="detail">{ref.sym.detail}</span>{/if}
      {/snippet}
    </TreeView>
  {/if}
</div>

<style>
  .outline {
    height: 100%;
    overflow: auto;
    font-family: var(--font-mono);
  }
  .name {
    color: var(--syn-key);
    font-size: var(--fs-sm);
  }
  .detail {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .doc {
    display: inline-grid;
    place-items: center;
    min-width: 16px;
    height: 16px;
    font-family: var(--font-ui);
    font-size: 10px;
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: var(--r-sm);
  }
  .hint {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-3);
    font-family: var(--font-ui);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>
