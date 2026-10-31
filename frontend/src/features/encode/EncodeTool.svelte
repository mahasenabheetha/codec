<script lang="ts">
  import { toolById } from '../../lib/tools'
  import TabbedTool from '../shared/TabbedTool.svelte'
  import ChecksumTab from './ChecksumTab.svelte'
  import HashTab from './HashTab.svelte'
  import HexTab from './HexTab.svelte'
  import HtpasswdTab from './HtpasswdTab.svelte'
  import PasswordTab from './PasswordTab.svelte'
  import SecretsTab from './SecretsTab.svelte'
  import UrlTab from './UrlTab.svelte'
  import UuidTab from './UuidTab.svelte'

  let { active }: { active: boolean } = $props()
</script>

<TabbedTool tool={toolById('encode')!} {active} tabs={{ url: UrlTab, hex: HexTab, hash: HashTab, checksum: ChecksumTab, password: PasswordTab, secrets: SecretsTab, uuid: UuidTab, htpasswd: HtpasswdTab }} />
