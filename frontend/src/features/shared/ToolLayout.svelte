<script lang="ts" module>
  /** Context key for a work-tab strip that sub-tab pages show. */
  export const workTabsStripKey = Symbol('work-tabs-strip')
</script>

<script lang="ts">
  import { getContext, type Snippet } from 'svelte'
  import { hueVar, type ToolDef } from '../../lib/tools'
  import SplitPane from '../../lib/components/SplitPane.svelte'

  // Standard tool page: title + toolbar, then input and output panes side
  // by side, or one column (body) for tools whose input is a line or a
  // small form.
  interface Props {
    tool: ToolDef
    controls?: Snippet
    actions?: Snippet
    input?: Snippet
    output?: Snippet
    body?: Snippet
    /** Work tabs (decision 97): the strip, between the toolbar and the panes. */
    tabs?: Snippet
  }

  let { tool, controls, actions, input, output, body, tabs: own }: Props = $props()
  // A tool whose sub-tabs sit inside work tabs (JSON & YAML) gives the
  // strip through context, so every sub-tab's page shows it.
  const tabs = $derived(own ?? getContext<Snippet | undefined>(workTabsStripKey))
</script>

<div class="tool hued" style:--tool={hueVar(tool.hue)}>
  <header class="toolbar">
    <div class="title">
      <span class="icon"><tool.icon size={16} strokeWidth={1.75} /></span>
      <h1>{tool.title}</h1>
    </div>
    {#if controls}<div class="controls">{@render controls()}</div>{/if}
    <div class="spacer"></div>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </header>
  {#if tabs}{@render tabs()}{/if}
  {#if body}
    <div class="body single">{@render body()}</div>
  {:else if input && output}
    <div class="body">
      <SplitPane id={tool.id} first={input} second={output} />
    </div>
  {/if}
</div>

<style>
  .tool {
    height: 100%;
    display: flex;
    flex-direction: column;
    padding: var(--s-3) var(--s-4) var(--s-4);
    gap: var(--s-3);
  }
  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3) var(--s-4);
    min-height: var(--control-h);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .icon {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    color: var(--tool-accent);
    background: var(--tool-accent-soft);
    border-radius: var(--r-md);
  }
  h1 {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.01em;
  }
  .controls,
  .actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .spacer {
    flex: 1;
  }
  .body {
    flex: 1;
    min-height: 0;
  }
  /* One column: the page scrolls; content keeps a readable width. */
  .body.single {
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
</style>
