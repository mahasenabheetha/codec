<script lang="ts">
  import { setContext, untrack, type Snippet } from 'svelte'
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import { workTabsStripKey } from '../shared/ToolLayout.svelte'
  import ConvertTab from './ConvertTab.svelte'
  import DiffTab from './DiffTab.svelte'
  import FormatTab from './FormatTab.svelte'
  import PathsTab from './PathsTab.svelte'
  import QueryTab from './QueryTab.svelte'
  import SchemaTab from './SchemaTab.svelte'
  import TreeTab from './TreeTab.svelte'
  import { provideJsonDoc } from './doc.svelte'
  import type { JsonDocTab } from './tabs.svelte'

  // One JSON & YAML work tab: its document, looked at through every
  // sub-tab, each mounted for this tab only (so their inputs are its own).
  let { tab, active, strip }: { tab: JsonDocTab; active: boolean; strip: Snippet } = $props()

  provideJsonDoc(untrack(() => tab.doc)) // a document component lives with one tab (keyed)
  setContext(workTabsStripKey, untrack(() => strip))
</script>

<TabbedTool
  tool={toolById('json')!}
  {active}
  tabs={{ format: FormatTab, tree: TreeTab, convert: ConvertTab, diff: DiffTab, paths: PathsTab, schema: SchemaTab, query: QueryTab }}
/>
