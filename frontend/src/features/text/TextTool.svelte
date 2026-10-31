<script lang="ts">
  import { toolById } from '../../lib/tools'
  import PlannedTab from '../shared/PlannedTab.svelte'
  import TabbedTool from '../shared/TabbedTool.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('text')!} {active} tabs={{ diff: PlannedTab, case: PlannedTab }} />
