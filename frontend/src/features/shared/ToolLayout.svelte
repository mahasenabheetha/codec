<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { ToolDef } from '../../lib/tools'
  import SplitPane from '../../lib/components/SplitPane.svelte'

  // Standard tool page: title + toolbar, then input and output panes.
  interface Props {
    tool: ToolDef
    controls?: Snippet
    actions?: Snippet
    input: Snippet
    output: Snippet
  }

  let { tool, controls, actions, input, output }: Props = $props()
</script>

<div class="tool">
  <header class="toolbar">
    <div class="title">
      <span class="icon"><tool.icon size={16} strokeWidth={1.75} /></span>
      <h1>{tool.title}</h1>
    </div>
    {#if controls}<div class="controls">{@render controls()}</div>{/if}
    <div class="spacer"></div>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </header>
  <div class="body">
    <SplitPane id={tool.id} first={input} second={output} />
  </div>
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
    color: var(--accent);
    background: var(--accent-soft);
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
</style>
