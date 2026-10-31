<script lang="ts">
  import CalendarCheck from '@lucide/svelte/icons/calendar-check'
  import type { ToolDef } from '../../lib/tools'
  import PlannedTab from './PlannedTab.svelte'

  let { tool }: { tool: ToolDef; active: boolean } = $props()
</script>

<PlannedTab
  {tool}
  icon={CalendarCheck}
  title="Today"
  description="Today's focus sessions, minutes and breaks, kept only until midnight. Coming with the focus timer."
/>
