<script lang="ts">
  import { toolById } from '../../lib/tools'
  import PlannedTab from '../shared/PlannedTab.svelte'
  import TabbedTool from '../shared/TabbedTool.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('network')!} {active} tabs={{ cidr: PlannedTab, ip: PlannedTab, url: PlannedTab, http: PlannedTab }} />
