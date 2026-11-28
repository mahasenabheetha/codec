<script lang="ts" module>
  /** Context key for a work-tab strip that sub-tab pages show. */
  export const workTabsStripKey = Symbol('work-tabs-strip')
</script>

<script lang="ts">
  import { getContext, type Snippet } from 'svelte'
  import Columns2 from '@lucide/svelte/icons/columns-2'
  import Rows2 from '@lucide/svelte/icons/rows-2'
  import { hueVar, type ToolDef } from '../../lib/tools'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import { narrow } from '../../lib/stores/narrow.svelte'
  import { paneLayout } from './panes.svelte'

  // Standard tool page: title + toolbar, then input and output panes,
  // resizable, side by side or one over the other (the button at the
  // end of the toolbar; remembered per tool), or one column (body) for
  // tools whose input is a line or a small form. In a narrow window the
  // panes stack and the controls take a second row with the choice.
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

  // Side by side needs this much room (SplitPane's own default).
  const ROOM = 760
  let width = $state(0)
  const split = $derived(!body && !!input && !!output)
  const choice = $derived(paneLayout.of(tool.id))
  const autoStacked = $derived(narrow.current || (width > 0 && width < ROOM))
  const stacked = $derived(choice === 'column' || (choice === 'auto' && autoStacked))
  // Stored as auto when auto already gives that layout, so it follows
  // the window again later.
  const choose = (s: boolean) => paneLayout.set(tool.id, s === autoStacked ? 'auto' : s ? 'column' : 'row')
</script>

<div class="tool hued" style:--tool={hueVar(tool.hue)}>
  <header class="toolbar">
    <div class="title">
      <span class="icon"><tool.icon size={16} /></span>
      <h1>{tool.title}</h1>
    </div>
    {#if controls && !narrow.current}<div class="controls">{@render controls()}</div>{/if}
    <div class="spacer"></div>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
    {#if split && !narrow.current}
      <IconButton
        icon={stacked ? Columns2 : Rows2}
        label={stacked ? 'Input beside output' : 'Input over output'}
        size="sm"
        onclick={() => choose(!stacked)}
      />
    {/if}
    {#if narrow.current && (controls || split)}
      <div class="second">
        {#if controls}<div class="controls">{@render controls()}</div>{/if}
        {#if split}
          <span class="layout">
            <SegmentedControl
              soft
              label="Layout"
              value={stacked ? 'column' : 'row'}
              options={[
                { value: 'row', label: 'Beside', icon: Columns2, title: 'Input beside output' },
                { value: 'column', label: 'Stacked', icon: Rows2, title: 'Input over output' },
              ]}
              onchange={(v) => choose(v === 'column')}
            />
          </span>
        {/if}
      </div>
    {/if}
  </header>
  {#if tabs}{@render tabs()}{/if}
  {#if body}
    <div class="body single">{@render body()}</div>
  {:else if input && output}
    <div class="body" bind:clientWidth={width}>
      <SplitPane id={tool.id} direction={stacked ? 'column' : 'row'} first={input} second={output} />
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
  /* Narrow: the title and actions on top, the controls and the layout
     choice on a row of their own. */
  .second {
    flex: 1 0 100%;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-4);
    min-width: 0;
  }
  .layout {
    margin-left: auto;
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
