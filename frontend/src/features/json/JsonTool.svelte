<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import ConvertTab from './ConvertTab.svelte'
  import DiffTab from './DiffTab.svelte'
  import FormatTab from './FormatTab.svelte'
  import PathsTab from './PathsTab.svelte'
  import QueryTab from './QueryTab.svelte'
  import SchemaTab from './SchemaTab.svelte'
  import TreeTab from './TreeTab.svelte'

  // JSON & YAML (3.7.0): one document pasted once and looked at every
  // way, plus a diff of two.
  let { active }: { active: boolean } = $props()
</script>

<TabbedTool
  tool={toolById('json')!}
  {active}
  tabs={{ format: FormatTab, tree: TreeTab, convert: ConvertTab, diff: DiffTab, paths: PathsTab, schema: SchemaTab, query: QueryTab }}
/>
