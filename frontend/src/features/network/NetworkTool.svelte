<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import CidrTab from './CidrTab.svelte'
  import HttpTab from './HttpTab.svelte'
  import IpTab from './IpTab.svelte'
  import UrlBuilderTab from './UrlBuilderTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('network')!} {active} tabs={{ cidr: CidrTab, ip: IpTab, url: UrlBuilderTab, http: HttpTab }} />
