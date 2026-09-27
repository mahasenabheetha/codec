<script lang="ts">
  import type { Snippet } from 'svelte'

  // The layout of a lens view: facts and problems on the side, the
  // main view (a graph, a tree, tables), and the selected item's
  // details — three columns when wide, stacked when narrow. Children
  // share the small vocabulary styled below (h3, .note, .mono, .link,
  // dl, .tbl, .muted, .count, .bar).
  interface Props {
    side: Snippet
    main: Snippet
    detail: Snippet
    sideLabel: string
    detailLabel: string
  }

  let { side, main, detail, sideLabel, detailLabel }: Props = $props()
</script>

<div class="lens">
  <div class="grid">
    <aside class="side" aria-label={sideLabel}>{@render side()}</aside>
    <main class="main">{@render main()}</main>
    <aside class="detail" aria-label={detailLabel}>{@render detail()}</aside>
  </div>
</div>

<style>
  .lens {
    height: 100%;
    min-height: 0;
    container-type: inline-size;
  }
  .grid {
    height: 100%;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(200px, 260px) minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr) minmax(0, 50%);
    grid-template-areas: 'side main' 'side detail';
  }
  @container (min-width: 1100px) {
    .grid {
      grid-template-columns: minmax(240px, 300px) minmax(0, 1fr) minmax(360px, 460px);
      grid-template-rows: minmax(0, 1fr);
      grid-template-areas: 'side main detail';
    }
    .detail {
      border-top: none !important;
      border-left: 1px solid var(--border);
    }
  }
  @container (max-width: 560px) {
    .grid {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: auto minmax(300px, 1fr) auto;
      grid-template-areas: 'side' 'main' 'detail';
      overflow: auto;
    }
  }
  .side,
  .detail {
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
    background: var(--bg-1);
  }
  .side {
    grid-area: side;
    border-right: 1px solid var(--border);
  }
  .detail {
    grid-area: detail;
    border-top: 1px solid var(--border);
  }
  .main {
    grid-area: main;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  /* The shared vocabulary of lens views. */
  .lens :global(section) {
    margin-bottom: var(--s-5);
  }
  .lens :global(h3) {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .detail :global(h3) {
    margin-top: var(--s-4);
  }
  .lens :global(.count) {
    margin-left: var(--s-1);
    color: var(--fg-1);
  }
  .lens :global(.note) {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow-wrap: anywhere;
  }
  .lens :global(.note.ok) {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    color: var(--ok);
    list-style: none;
  }
  .lens :global(.mono) {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .lens :global(.muted) {
    color: var(--fg-2);
    font-size: var(--fs-xs);
  }
  .lens :global(.link) {
    padding: 0;
    color: var(--accent);
    background: none;
    border: none;
    text-align: left;
    font-size: inherit;
    overflow-wrap: anywhere;
  }
  .lens :global(.link:hover) {
    text-decoration: underline;
  }
  .lens :global(dl) {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 3px var(--s-3);
    margin: 0;
    font-size: var(--fs-sm);
  }
  .lens :global(dt) {
    color: var(--fg-2);
    white-space: nowrap;
  }
  .lens :global(dd) {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .lens :global(.bar) {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .lens :global(.tbl) {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  .lens :global(.tbl th),
  .lens :global(.tbl td) {
    padding: var(--s-1) var(--s-2) var(--s-1) 0;
    text-align: left;
    vertical-align: top;
    border-bottom: 1px solid var(--border);
    overflow-wrap: anywhere;
  }
  .lens :global(.tbl thead th) {
    font-size: var(--fs-xs);
    font-weight: var(--fw-medium);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .lens :global(.plain) {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .lens :global(.problems) {
    margin: 0;
    padding: 0;
  }
  .lens :global(.dhead) {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  .lens :global(.dname) {
    padding: 0;
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
    background: none;
    border: none;
    text-align: left;
    overflow-wrap: anywhere;
  }
  .lens :global(.dname:hover) {
    color: var(--accent);
  }
  .lens :global(.field) {
    flex: 1;
    min-width: 0;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
</style>
