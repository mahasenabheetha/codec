<script lang="ts">
  import Timer from '@lucide/svelte/icons/timer'
  import type { ToolDef } from '../../lib/tools'
  import PlannedTab from './PlannedTab.svelte'
  import SoundSettings from './SoundSettings.svelte'

  let { tool }: { tool: ToolDef; active: boolean } = $props()
</script>

<PlannedTab
  {tool}
  icon={Timer}
  title="Focus timer"
  description="Focus sessions and breaks on a large ring: Classic 25/5, Deep work 50/10, Flow 90/20 or your own lengths. Coming in the next step; the sound it rings is below."
>
  <SoundSettings />
</PlannedTab>
