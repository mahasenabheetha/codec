<script lang="ts">
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Info from '@lucide/svelte/icons/info'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import { lint } from './lint.svelte'

  // One finding: what is wrong, how to fix it, where, the rule, and (on
  // demand) why it matters. Lint rules can be turned off from here.
  interface Props {
    severity: 'error' | 'warning' | 'info'
    message: string
    hint?: string
    why?: string
    code?: string
    where?: string // e.g. "12:3"
    onjump: () => void
    tag?: 'li' | 'div' // div inside a windowed list, which has no <ul>
  }

  let { severity, message, hint, why, code, where, onjump, tag = 'li' }: Props = $props()
  let open = $state(false)
  const icons = { error: CircleX, warning: TriangleAlert, info: Info }
  const Icon = $derived(icons[severity])
  const rule = $derived(code ? lint.rule(code) : undefined)
</script>

<svelte:element this={tag} class="item {severity}">
  <button type="button" class="main" onclick={onjump}>
    <span class="icon"><Icon size={15} /></span>
    <span class="text">
      <span class="msg">{message}</span>
      {#if hint}<span class="hint-line">{hint}</span>{/if}
      {#if where || code}
        <span class="place">
          {#if where}<span class="where">{where}</span>{/if}
          {#if code}<Badge mono>{code}</Badge>{/if}
        </span>
      {/if}
    </span>
  </button>
  {#if why || rule}
    <div class="extra">
      {#if why}
        <Button size="sm" icon={Info} aria-expanded={open} onclick={() => (open = !open)}>{open ? 'Hide why' : 'Why?'}</Button>
      {/if}
      {#if rule}
        <span class="off">
          <Button size="sm" variant="ghost" title="Change it back in Settings" onclick={() => lint.turnOff(rule.id)}>Turn this rule off</Button>
        </span>
      {/if}
    </div>
    {#if open && why}<p class="why">{why}</p>{/if}
  {/if}
</svelte:element>

<style>
  .item {
    --icon-col: 18px;
    list-style: none;
    border-bottom: 1px solid var(--border);
  }
  .item:hover {
    background: var(--bg-2);
  }
  .main {
    display: grid;
    grid-template-columns: var(--icon-col) minmax(0, 1fr);
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-2) var(--s-3);
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
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    min-width: 0;
  }
  .msg {
    font-weight: var(--fw-medium);
  }
  .hint-line {
    color: var(--fg-1);
  }
  .place {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .where {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .extra {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: 0 var(--s-3) var(--s-2) calc(var(--s-3) + var(--icon-col) + var(--s-2));
  }
  /* Turning a rule off is there when wanted, not in every row. */
  .off {
    opacity: 0;
  }
  .item:hover .off,
  .off:focus-within {
    opacity: 1;
  }
  .why {
    margin: 0;
    padding: 0 var(--s-3) var(--s-2) calc(var(--s-3) + var(--icon-col) + var(--s-2));
    font-size: var(--fs-sm);
    color: var(--fg-1);
    line-height: 1.45;
  }
</style>
