<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import CronTab from './CronTab.svelte'
  import TimestampTab from './TimestampTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('time')!} {active} tabs={{ timestamp: TimestampTab, cron: CronTab }} />
