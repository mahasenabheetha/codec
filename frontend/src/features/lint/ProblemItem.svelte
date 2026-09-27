<script lang="ts">
  import CircleX from '@lucide/svelte/icons/circle-x'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import Info from '@lucide/svelte/icons/info'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import { lint } from './lint.svelte'

  // One finding: what is wrong, how to fix it, and (on demand) why it
  // matters. Lint rules can be turned off from here.
  interface Props {
    severity: 'error' | 'warning' | 'info'
    message: string
    hint?: string
    why?: string
    code?: string
    where?: string // e.g. "12:3"
    onjump: () => void
  }

  let { severity, message, hint, why, code, where, onjump }: Props = $props()
  let open = $state(false)
  const icons = { error: CircleX, warning: TriangleAlert, info: Info }
  const Icon = $derived(icons[severity])
  const rule = $derived(code ? lint.rule(code) : undefined)
</script>

<li class="item {severity}">
  <button type="button" class="main" onclick={onjump}>
    <span class="icon"><Icon size={14} strokeWidth={1.75} /></span>
    <span class="text">
      <span class="msg">{message}</span>
      {#if hint}<span class="hint-line">{hint}</span>{/if}
    </span>
    {#if where}<span class="where">{where}</span>{/if}
  </button>
  {#if why || rule}
    <div class="extra">
      {#if why}
        <button type="button" class="link" aria-expanded={open} onclick={() => (open = !open)}>{open ? 'Hide why' : 'Why?'}</button>
      {/if}
      {#if code}<span class="code">{code}</span>{/if}
      {#if rule}
        <button type="button" class="link off" title="Turn this rule off (change it back in Settings)" onclick={() => lint.turnOff(rule.id)}>
          <EyeOff size={12} strokeWidth={1.75} /> Turn off
        </button>
      {/if}
    </div>
    {#if open && why}<p class="why">{why}</p>{/if}
  {/if}
</li>

<style>
  .item {
    list-style: none;
    border-radius: var(--r-sm);
  }
  .item:hover {
    background: var(--bg-2);
  }
  .main {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-2) var(--s-2) var(--s-1);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .icon {
    display: grid;
    padding-top: 1px;
  }
  .error .icon {
    color: var(--err);
  }
  .warning .icon {
    color: var(--warn);
  }
  .info .icon {
    color: var(--info);
  }
  .text {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .hint-line {
    color: var(--fg-1);
  }
  .where {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .extra {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: 0 var(--s-2) var(--s-2) calc(var(--s-2) * 2 + 14px);
    font-size: var(--fs-xs);
  }
  .code {
    font-family: var(--font-mono);
    color: var(--fg-2);
  }
  .link {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 0;
    font-size: var(--fs-xs);
    color: var(--accent);
    background: none;
    border: none;
  }
  .link:hover {
    text-decoration: underline;
  }
  .link.off {
    color: var(--fg-2);
    opacity: 0;
  }
  .item:hover .link.off,
  .link.off:focus-visible {
    opacity: 1;
  }
  .why {
    margin: 0;
    padding: 0 var(--s-3) var(--s-2) calc(var(--s-2) * 2 + 14px);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    line-height: 1.45;
  }
</style>
