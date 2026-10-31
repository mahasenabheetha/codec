<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import MakeTab from './MakeTab.svelte'
  import ReadTab from './ReadTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('qr')!} {active} tabs={{ make: MakeTab, read: ReadTab }} />
