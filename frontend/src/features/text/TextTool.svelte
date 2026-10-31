<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import CaseTab from './CaseTab.svelte'
  import DiffTab from './DiffTab.svelte'
  import FakeTab from './FakeTab.svelte'
  import TableTab from './TableTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('text')!} {active} tabs={{ diff: DiffTab, case: CaseTab, table: TableTab, fake: FakeTab }} />
