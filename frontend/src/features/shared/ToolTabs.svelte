<script lang="ts">
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { currentTab, type ToolDef } from '../../lib/tools'

  // A tool's sub-tabs, for its toolbar. The choice is remembered.
  let { tool }: { tool: ToolDef } = $props()

  const options = $derived((tool.tabs ?? []).map((t) => ({ value: t.id, label: t.title, title: t.description })))
</script>

<SegmentedControl
  {options}
  value={currentTab(tool, layout.toolTab(tool.id))}
  label={tool.title}
  onchange={(v) => layout.setToolTab(tool.id, v)}
/>
