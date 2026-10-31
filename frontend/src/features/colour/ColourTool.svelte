<script lang="ts">
  import { untrack } from 'svelte'
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Copy from '@lucide/svelte/icons/copy'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { contrast, formats, luminance, over, parseColour, scale, toHex, wcag, type RGB } from '../../lib/utils/colour'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // Colours (3.8.0), worked out here in the browser: a colour written any
  // common way in every format, its WCAG contrast against a second one,
  // and a scale of shades. Nothing is stored or sent.
  let { active }: { active: boolean } = $props()
  const tool = toolById('colour')!

  let text = $state('#4f46e5')
  let bgText = $state('#ffffff')
  let field = $state<HTMLInputElement>()

  // A colour handed over by smart paste.
  $effect(() => {
    void handoff.pending
    const v = handoff.take('colour', '')
    if (v !== null) untrack(() => (text = v))
  })

  // The last colour that read, so a half-typed value doesn't blank the page.
  let last = $state<RGB>({ r: 79, g: 70, b: 229, a: 1 })
  let lastBg = $state<RGB>({ r: 255, g: 255, b: 255, a: 1 })
  const parsed = $derived(parseColour(text))
  const parsedBg = $derived(parseColour(bgText))
  $effect(() => {
    if (parsed) last = parsed
  })
  $effect(() => {
    if (parsedBg) lastBg = parsedBg
  })
  const c = $derived(parsed ?? last)
  const bg = $derived({ ...(parsedBg ?? lastBg), a: 1 })

  const rows = $derived(formats(c))
  const ratio = $derived(contrast(c, bg))
  const checks = $derived(wcag(ratio))
  const shades = $derived(scale(c))
  const css = (x: RGB) => `rgb(${x.r} ${x.g} ${x.b} / ${x.a})`
  // Text drawn on a swatch: whichever of black or white reads better.
  const ink = (x: RGB) => (luminance(x) > 0.179 ? '#000' : '#fff')
  const opaqueHex = (x: RGB) => toHex({ ...x, a: 1 })
  // A translucent colour over the swatch's grey checkerboard, for its ink.
  const onChecker = (x: RGB) => (x.a < 1 ? over(x, { r: 128, g: 128, b: 128, a: 1 }) : x)
  // The colour laid over a checkerboard, which shows through when translucent.
  const swatchBg = (x: RGB) => `linear-gradient(${css(x)}, ${css(x)}), repeating-conic-gradient(#8884 0 25%, transparent 0 50%) 0 0 / 16px 16px`

  function swap() {
    ;[text, bgText] = [bgText, text]
  }

  toolShortcuts(() => active, { run: () => field?.focus(), copy: () => copyText(rows[0].value, 'HEX'), clear: () => ((text = ''), field?.focus()), swap })
  toolStatus(() => active, () => {
    if (!parsed && text.trim()) return { text: 'Not a colour: try #4f46e5, rgb(79 70 229), hsl(243 75% 59%) or a name', tone: 'err' }
    const pass = checks.filter((k) => k.pass).length
    return { text: `${rows[0].value} · ${rows[5].value} · ${ratio.toFixed(2)}:1 on ${opaqueHex(bg)} · passes ${pass} of ${checks.length}`, tone: ratio >= 4.5 ? 'ok' : 'warn' }
  })
</script>

<ToolLayout {tool}>
  {#snippet body()}
    <div class="grid">
      <Pane title="Colour" grow={false}>
        <div class="colour">
          <div class="swatch" style:background={swatchBg(c)} style:color={ink(onChecker(c))}>
            <span>{rows[0].value}</span>
            <input type="color" value={opaqueHex(c)} oninput={(e) => (text = (e.currentTarget as HTMLInputElement).value)} aria-label="Pick a colour" title="Pick a colour" />
          </div>
          <div class="side">
            <LineInput bind:value={text} bind:field label="Colour" placeholder="#4f46e5, rgb(79 70 229), hsl(…), oklch(…) or a name" invalid={!parsed && !!text.trim()} />
            <dl class="formats">
              {#each rows as r (r.label)}
                <div class="fmt">
                  <dt>{r.label}</dt>
                  <dd><code>{r.value}</code></dd>
                  <IconButton icon={Copy} tone="copy" label="Copy {r.label}" size="sm" onclick={() => copyText(r.value.replace(/^≈ /, ''), r.label)} />
                </div>
              {/each}
            </dl>
          </div>
        </div>
      </Pane>

      <Pane title="Contrast" grow={false}>
        {#snippet meta()}<span>WCAG 2</span>{/snippet}
        {#snippet actions()}
          <IconButton icon={ArrowLeftRight} tone="swap" label="Swap text and background" shortcut="Alt+S" size="sm" onclick={swap} />
        {/snippet}
        <div class="contrast">
          <div class="bg-row">
            <span class="label">On background</span>
            <input class="text" bind:value={bgText} aria-label="Background colour" spellcheck="false" class:bad={!parsedBg && !!bgText.trim()} />
            <input type="color" class="pick" value={opaqueHex(bg)} oninput={(e) => (bgText = (e.currentTarget as HTMLInputElement).value)} aria-label="Pick the background" />
          </div>
          <div class="sample" style:background={css(bg)} style:color={css(c)}>
            <p class="large">Large text 24 px</p>
            <p>Body text at 15 px, as people read most of a page.</p>
            <span class="ui" style:border-color={css(c)}>A button outline</span>
          </div>
          <div class="ratio">
            <strong>{ratio.toFixed(2)}<span>:1</span></strong>
            <ul>
              {#each checks as k (k.name)}
                <li class:pass={k.pass} title={k.title}>
                  {#if k.pass}<CircleCheck size={14} strokeWidth={2} />{:else}<CircleX size={14} strokeWidth={2} />{/if}
                  {k.name} <span class="need">{k.need}:1</span>
                </li>
              {/each}
            </ul>
          </div>
        </div>
      </Pane>

      <Pane title="Shades" grow={false}>
        {#snippet meta()}<span>in OKLCH, so the steps look even</span>{/snippet}
        <div class="shades">
          {#each shades as s (s.step)}
            <button type="button" class="shade" style:background={css(s.colour)} style:color={ink(s.colour)} title="Copy {opaqueHex(s.colour)} · double-click to use it" onclick={() => copyText(opaqueHex(s.colour), `${s.step}`)} ondblclick={() => (text = opaqueHex(s.colour))}>
              <span class="step">{s.step}</span>
              <span class="hex">{opaqueHex(s.colour)}</span>
            </button>
          {/each}
        </div>
      </Pane>
    </div>
  {/snippet}
</ToolLayout>

<style>
  .grid {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .colour {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-4);
    padding: var(--s-4);
  }
  .swatch {
    position: relative;
    width: 200px;
    min-height: 200px;
    display: flex;
    align-items: flex-end;
    padding: var(--s-3);
    font-family: var(--font-mono);
    font-weight: var(--fw-semibold);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .swatch input[type='color'] {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
  }
  .side {
    flex: 1;
    min-width: 280px;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .formats {
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .fmt {
    display: grid;
    /* The value as wide as it is, so Copy sits right after it. */
    grid-template-columns: 64px minmax(0, max-content) auto;
    justify-content: start;
    align-items: center;
    gap: var(--s-2);
    padding: 2px var(--s-2);
    border-radius: var(--r-sm);
  }
  .fmt:hover {
    background: var(--bg-3);
  }
  dt {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  dd {
    margin: 0;
    min-width: 0;
  }
  code {
    font-size: var(--fs-md);
    overflow-wrap: anywhere;
    user-select: all;
  }
  .contrast {
    display: grid;
    grid-template-columns: minmax(260px, 1fr) minmax(220px, auto);
    gap: var(--s-3) var(--s-4);
    padding: var(--s-4);
  }
  .bg-row {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .text {
    width: 220px;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .text:focus {
    outline: none;
    border-color: var(--accent);
  }
  .text.bad {
    border-color: var(--err);
  }
  .pick {
    width: 32px;
    height: var(--control-h);
    padding: 0;
    background: none;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    cursor: pointer;
  }
  .sample {
    padding: var(--s-4);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .sample p {
    margin: 0 0 var(--s-2);
    font-size: 15px;
  }
  .sample .large {
    font-size: 24px;
  }
  .ui {
    display: inline-block;
    padding: 2px var(--s-3);
    font-size: 13px;
    border: 2px solid;
    border-radius: var(--r-sm);
  }
  .ratio strong {
    display: block;
    font-size: 40px;
    line-height: 1.1;
    font-variant-numeric: tabular-nums;
  }
  .ratio strong span {
    font-size: var(--fs-lg);
    color: var(--fg-2);
  }
  .ratio ul {
    margin: var(--s-2) 0 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .ratio li {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--err);
  }
  .ratio li.pass {
    color: var(--ok);
  }
  .need {
    color: var(--fg-2);
  }
  .shades {
    display: grid;
    grid-template-columns: repeat(11, minmax(56px, 1fr));
    gap: var(--s-1);
    padding: var(--s-4);
    overflow-x: auto;
  }
  .shade {
    height: 72px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    align-items: flex-start;
    padding: var(--s-2);
    font-family: var(--font-mono);
    border: none;
    border-radius: var(--r-md);
    cursor: pointer;
  }
  .shade:hover {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .step {
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
  }
  .hex {
    font-size: var(--fs-xs);
  }
</style>
