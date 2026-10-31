<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import BreaksTab from './BreaksTab.svelte'
  import FocusTab from './FocusTab.svelte'
  import TodayTab from './TodayTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('wellness')!} {active} tabs={{ focus: FocusTab, breaks: BreaksTab, today: TodayTab }} />
