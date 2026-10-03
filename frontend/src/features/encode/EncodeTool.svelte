<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import HashTab from './HashTab.svelte'
  import HexTab from './HexTab.svelte'
  import HtpasswdTab from './HtpasswdTab.svelte'
  import SecretsTab from './SecretsTab.svelte'
  import UrlTab from './UrlTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('encode')!} {active} tabs={{ url: UrlTab, hex: HexTab, hash: HashTab, secrets: SecretsTab, htpasswd: HtpasswdTab }} />
