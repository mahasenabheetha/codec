<script lang="ts">
  import Eye from '@lucide/svelte/icons/eye'
  import type { ToolDef } from '../../lib/tools'
  import PlannedTab from './PlannedTab.svelte'

  let { tool }: { tool: ToolDef; active: boolean } = $props()
</script>

<PlannedTab
  {tool}
  icon={Eye}
  title="Break reminders"
  description="Eye breaks, stretches, water and posture, each at its own interval, with a sound and a full-window break, a card or a toast. Coming soon."
/>
