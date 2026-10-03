<script lang="ts">
  import { tick } from 'svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { currentTab, type ToolDef } from '../../lib/tools'

  // A tool's sub-tabs, for its toolbar. The choice is remembered.
  let { tool }: { tool: ToolDef } = $props()

  let box = $state<HTMLElement>()
  const options = $derived((tool.tabs ?? []).map((t) => ({ value: t.id, label: t.title, title: t.description })))

  // Each sub-tab has its own toolbar, so choosing another hides the one
  // holding focus: move focus to the same control in the new sub-tab, so
  // arrow keys keep working.
  async function pick(id: string) {
    const focused = !!box?.contains(document.activeElement)
    const frame = box?.closest('[data-subtab]')?.parentElement
    layout.setToolTab(tool.id, id)
    if (!focused || !frame) return
    await tick()
    frame.querySelector<HTMLElement>(`[data-subtab="${id}"] .tool-tabs [aria-checked="true"]`)?.focus()
  }
</script>

<span class="tool-tabs" bind:this={box}>
  <SegmentedControl {options} value={currentTab(tool, layout.toolTab(tool.id))} label={tool.title} onchange={pick} />
</span>

<style>
  .tool-tabs {
    display: contents;
  }
</style>
