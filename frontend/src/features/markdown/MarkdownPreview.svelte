<script lang="ts" module>
  // Files whose remote images were let in, until codec is closed; never
  // stored (decision 19).
  const remoteAllowed = new Set<string>()
  // A heading to scroll to once that file's preview shows: a link like
  // guide.md#install, followed from another file.
  const pending = $state<{ path: string; id: string }>({ path: '', id: '' })
</script>

<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import ImageOff from '@lucide/svelte/icons/image-off'
  import Button from '../../lib/components/Button.svelte'
  import { getImage } from '../../lib/api/workspace'
  import { highlightToAsync } from '../../lib/components/syntax'
  import { openLink } from '../../lib/platform'
  import { layout } from '../../lib/stores/layout.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import { linkTarget, type Target } from './links'
  import { renderMarkdown, sanitizeSVG } from './render'
  import { drawMermaid } from './mermaid'
  import { theme } from '../../lib/stores/theme.svelte'
  import { toolColours } from '../../lib/stores/colour.svelte'

  // A Markdown file rendered (decision 84): the editor's text, so what-if
  // edits show. The app's routes live in the URL hash, so no link in here
  // may navigate the page: #heading scrolls within the preview, a link to
  // a file opens it in a codec tab, web links open in the browser.
  // Images in the folder load through the API; remote ones only after
  // "Load images", since they tell another server the file was opened.
  interface Props {
    text: string
    /** The file's workspace path, which relative links resolve against. */
    path: string
    /** Whether its tab is showing. */
    active: boolean
  }

  let { text, path, active }: Props = $props()

  let el = $state<HTMLDivElement>()
  const html = $derived(renderMarkdown(text))
  let allowRemote = $state(remoteAllowed.has(untrack(() => path)))
  let blocked = $state(0)

  // Object URLs of the folder's images, by path; kept across re-renders
  // so typing doesn't fetch them again.
  const images = new Map<string, Promise<string | null>>()
  function folderImage(p: string): Promise<string | null> {
    let u = images.get(p)
    if (!u) {
      u = getImage(p)
        .then(async (b) => (b.type === 'image/svg+xml' ? new Blob([sanitizeSVG(await b.text())], { type: b.type }) : b))
        .then((b) => URL.createObjectURL(b))
        .catch(() => null)
      images.set(p, u)
    }
    return u
  }
  onDestroy(() => images.forEach((u) => u.then((url) => url && URL.revokeObjectURL(url))))

  // Give each image its source once the HTML is in the page.
  $effect(() => {
    void html
    const allow = allowRemote
    if (!el) return
    let n = 0
    for (const img of el.querySelectorAll<HTMLImageElement>('img[data-src]')) {
      const t = linkTarget(img.dataset.src ?? '', path)
      if (t.kind === 'data') img.src = t.url
      else if (t.kind === 'web' && /^https?:/i.test(t.url)) {
        if (allow) img.src = t.url
        else {
          n++
          img.title = `Not loaded: ${t.url}`
        }
      } else if (t.kind === 'file') {
        folderImage(t.path).then((url) => {
          if (url) img.src = url
          else img.title = `Can't show ${t.path}: not an image in the open folder`
        })
      } else img.title = 'Not shown: only images in the open folder load'
    }
    blocked = n
  })

  // YAML, JSON and shell blocks coloured as in the editor; the code
  // comes from the element's text, so nothing in it becomes markup.
  $effect(() => {
    void html
    if (!el) return
    for (const code of el.querySelectorAll('pre > code[class*="language-"]')) {
      const lang = /(?:^|\s)language-(\S+)/.exec(code.className)?.[1]
      if (lang === 'mermaid') void diagram(code.parentElement!, code.textContent ?? '')
      else if (lang) void highlightToAsync(code, code.textContent ?? '', lang)
    }
    // Drawn ones again, in the theme now in use.
    for (const fig of el.querySelectorAll<HTMLElement>('figure.mermaid-block')) void diagram(fig, fig.dataset.src ?? '')
  })

  // A ```mermaid block becomes its diagram (3.10.3); one that doesn't
  // parse stays code, with the reason under it. The theme redraws it.
  async function diagram(pre: Element, src: string) {
    void theme.resolved
    void toolColours.on
    const r = await drawMermaid(src)
    if (!pre.isConnected) return
    if ('error' in r) {
      if (pre.nextElementSibling?.classList.contains('mermaid-error')) return
      const note = document.createElement('p')
      note.className = 'mermaid-error'
      note.textContent = `Mermaid couldn't draw this diagram: ${r.error.split('\n')[0]}`
      pre.after(note)
      return
    }
    const fig = document.createElement('figure')
    fig.className = 'mermaid-block'
    fig.dataset.src = src
    fig.innerHTML = r.svg // cleaned by drawMermaid
    pre.replaceWith(fig)
  }

  // Take a heading another file's link asked for once this tab shows.
  $effect(() => {
    void html
    if (!active || pending.path !== path) return
    const id = pending.id
    pending.path = ''
    scrollTo(id) // runs after the DOM update that shows this tab
  })

  function loadRemote() {
    remoteAllowed.add(path)
    allowRemote = true
  }

  function scrollTo(id: string) {
    el?.querySelector(`[id="user-content-${CSS.escape(id)}"]`)?.scrollIntoView({ block: 'start' })
  }

  function follow(t: Target, href: string) {
    switch (t.kind) {
      case 'anchor':
        scrollTo(t.id)
        break
      case 'web':
        openLink(t.url).catch((e) => toast(e instanceof Error ? e.message : String(e), 'err'))
        break
      case 'file':
        if (ws.byPath.has(t.path)) {
          if (t.id) Object.assign(pending, { path: t.path, id: t.id })
          layout.openFile(t.path)
        } else if (ws.files.some((f) => f.path.startsWith(t.path + '/'))) {
          layout.explorerOpen = true
          ws.reveal(t.path + '/')
        } else toast(`${t.path} is not in the open folder`, 'warn')
        break
      default:
        toast(`codec doesn't open links like ${href}`, 'warn')
    }
  }

  function onclick(e: MouseEvent) {
    const a = (e.target as Element).closest('a')
    if (!a || !el?.contains(a)) return
    e.preventDefault()
    if (a.hasAttribute('href')) follow(linkTarget(a.getAttribute('href') ?? '', path), a.getAttribute('href') ?? '')
  }

  // A middle click would open the link's address in a new browser tab.
  function onauxclick(e: MouseEvent) {
    if ((e.target as Element).closest('a')) e.preventDefault()
  }
</script>

<!-- Clicks come from the links inside, which take keys themselves. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="preview" bind:this={el} {onclick} {onauxclick}>
  {#if blocked && !allowRemote}
    <div class="blocked" role="status">
      <ImageOff size={14} />
      <span>{blocked} remote image{blocked === 1 ? '' : 's'} not loaded: showing them tells another server this file was opened.</span>
      <Button size="sm" onclick={loadRemote}>Load images</Button>
    </div>
  {/if}
  <article class="md">
    {@html html /* sanitised by renderMarkdown */}
  </article>
</div>

<style>
  .preview {
    flex: 1;
    min-height: 0;
    height: 100%;
    overflow: auto;
    background: var(--bg-2);
  }
  .blocked {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-1);
    border-bottom: 1px solid var(--border);
  }
  .blocked span {
    flex: 1;
    min-width: 0;
  }
  /* Colour (3.10.3): headings in three hues, h1 to h3, then the text
     colour; with Tool colours off (--tint-tool 0%) they are all the text
     colour, as the rest of the interface goes to one accent. Every hue
     is checked on this ground (bg-2) in each theme. */
  .md {
    --md-h1: color-mix(in oklab, var(--hue-violet) var(--tint-tool), var(--fg-0));
    --md-h2: color-mix(in oklab, var(--hue-indigo) var(--tint-tool), var(--fg-0));
    --md-h3: color-mix(in oklab, var(--hue-teal) var(--tint-tool), var(--fg-0));
    --md-mark: color-mix(in oklab, var(--hue-indigo) var(--tint-tool), var(--fg-2));
    max-width: 880px;
    margin: 0 auto;
    padding: var(--s-6) var(--s-8) var(--s-12);
    font-family: var(--font-ui);
    font-size: var(--fs-lg);
    line-height: 1.6;
    color: var(--fg-0);
    overflow-wrap: break-word;
  }
  .md > :global(:first-child) {
    margin-top: 0;
  }
  .md :global(:is(p, ul, ol, blockquote, table, pre)) {
    margin: 0 0 var(--s-4);
  }
  .md :global(:is(h1, h2, h3, h4, h5, h6)) {
    position: relative;
    margin: var(--s-6) 0 var(--s-3);
    font-weight: var(--fw-semibold);
    line-height: 1.25;
    scroll-margin-top: var(--s-4);
  }
  .md :global(h1) {
    font-size: var(--fs-3xl);
    padding-bottom: var(--s-2);
    color: var(--md-h1);
    border-bottom: 2px solid color-mix(in oklab, var(--md-h1) 40%, var(--border));
  }
  .md :global(h2) {
    font-size: var(--fs-2xl);
    padding-bottom: var(--s-2);
    color: var(--md-h2);
    border-bottom: 1px solid color-mix(in oklab, var(--md-h2) 30%, var(--border));
  }
  .md :global(h3) {
    font-size: var(--fs-xl);
    color: var(--md-h3);
  }
  .md :global(:is(h4, h5, h6)) {
    font-size: var(--fs-lg);
  }
  .md :global(h6) {
    color: var(--fg-1);
  }
  .md :global(.anchor) {
    margin-left: var(--s-2);
    color: var(--fg-2);
    text-decoration: none;
    opacity: 0;
    transition: opacity var(--dur) var(--ease);
  }
  .md :global(:is(h1, h2, h3, h4, h5, h6):hover .anchor),
  .md :global(.anchor:focus-visible) {
    opacity: 1;
  }
  .md :global(a[href]) {
    color: var(--accent);
    text-decoration: none;
  }
  .md :global(a[href]:hover) {
    text-decoration: underline;
    text-underline-offset: 0.2em;
  }
  /* A heading's own anchor keeps its quiet grey. */
  .md :global(a.anchor[href]) {
    color: var(--fg-2);
  }
  .md :global(li::marker) {
    color: var(--md-mark);
  }
  .md :global(ol > li::marker) {
    font-weight: var(--fw-semibold);
  }
  .md :global(:is(ul, ol)) {
    padding-left: 2em;
  }
  .md :global(li + li) {
    margin-top: var(--s-1);
  }
  .md :global(li > :is(ul, ol)) {
    margin: var(--s-1) 0 0;
  }
  .md :global(li:has(> input[type='checkbox'])) {
    list-style: none;
  }
  .md :global(li > input[type='checkbox']) {
    appearance: none;
    position: relative;
    width: 1em;
    height: 1em;
    margin: 0 var(--s-2) 0 -1.4em;
    vertical-align: -0.15em;
    border: 1.5px solid var(--fg-2);
    border-radius: var(--r-sm);
  }
  .md :global(li > input[type='checkbox']:checked) {
    background: var(--ok);
    border-color: var(--ok);
  }
  /* The tick: two borders of a box, turned. */
  .md :global(li > input[type='checkbox']:checked::after) {
    content: '';
    position: absolute;
    left: 30%;
    top: 8%;
    width: 30%;
    height: 58%;
    border: solid var(--bg-2);
    border-width: 0 2px 2px 0;
    transform: rotate(45deg);
  }
  .md :global(blockquote) {
    padding: var(--s-2) var(--s-4);
    color: var(--fg-1);
    background: var(--md-quote-bg);
    border-left: 3px solid var(--accent);
    border-radius: 0 var(--r-md) var(--r-md) 0;
  }
  .md :global(blockquote > :last-child) {
    margin-bottom: 0;
  }
  .md :global(code) {
    padding: 0.15em 0.4em;
    font-family: var(--font-mono);
    font-size: 0.86em;
    background: var(--md-code-bg);
    border-radius: var(--r-sm);
  }
  /* Callouts: GitHub's > [!NOTE], [!TIP], [!IMPORTANT], [!WARNING] and
     [!CAUTION], in the status colours (Important in violet, as GitHub
     has it), each with its icon and its name, so not by colour alone. */
  .md :global(.alert) {
    margin: 0 0 var(--s-4);
    padding: var(--s-2) var(--s-4) var(--s-3);
    background: var(--alert-bg);
    border-left: 3px solid var(--alert);
    border-radius: 0 var(--r-md) var(--r-md) 0;
  }
  .md :global(.alert > :last-child) {
    margin-bottom: 0;
  }
  .md :global(.alert-title) {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0 0 var(--s-1);
    font-weight: var(--fw-semibold);
    color: var(--alert);
  }
  .md :global(.alert-title::before) {
    content: '';
    flex: none;
    width: 1.05em;
    height: 1.05em;
    background: currentColor;
    mask: var(--alert-icon) center / contain no-repeat;
  }
  .md :global(.alert-note) {
    --alert: var(--info);
    --alert-bg: var(--md-note-bg);
    --alert-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='1.9' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='10'/%3E%3Cpath d='M12 16v-4'/%3E%3Cpath d='M12 8h.01'/%3E%3C/svg%3E");
  }
  .md :global(.alert-tip) {
    --alert: var(--ok);
    --alert-bg: var(--md-tip-bg);
    --alert-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='1.9' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.7 1.3 1.5 1.5 2.5'/%3E%3Cpath d='M9 18h6'/%3E%3Cpath d='M10 22h4'/%3E%3C/svg%3E");
  }
  .md :global(.alert-important) {
    --alert: var(--hue-violet);
    --alert-bg: var(--md-important-bg);
    --alert-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='1.9' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z'/%3E%3Cpath d='M12 15h.01'/%3E%3Cpath d='M12 7v4'/%3E%3C/svg%3E");
  }
  .md :global(.alert-warning) {
    --alert: var(--warn);
    --alert-bg: var(--md-warning-bg);
    --alert-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='1.9' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3'/%3E%3Cpath d='M12 9v4'/%3E%3Cpath d='M12 17h.01'/%3E%3C/svg%3E");
  }
  .md :global(.alert-caution) {
    --alert: var(--err);
    --alert-bg: var(--md-caution-bg);
    --alert-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='1.9' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M12 16h.01'/%3E%3Cpath d='M12 8v4'/%3E%3Cpath d='M15.312 2a2 2 0 0 1 1.414.586l4.688 4.688A2 2 0 0 1 22 8.688v6.624a2 2 0 0 1-.586 1.414l-4.688 4.688a2 2 0 0 1-1.414.586H8.688a2 2 0 0 1-1.414-.586l-4.688-4.688A2 2 0 0 1 2 15.312V8.688a2 2 0 0 1 .586-1.414l4.688-4.688A2 2 0 0 1 8.688 2z'/%3E%3C/svg%3E");
  }
  .md :global(pre) {
    padding: var(--s-3) var(--s-4);
    overflow: auto;
    font-size: var(--fs-md);
    line-height: var(--lh-code);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .md :global(pre code) {
    padding: 0;
    font-size: inherit;
    background: none;
    border-radius: 0;
  }
  .md :global(table) {
    display: block;
    width: max-content;
    max-width: 100%;
    overflow: auto;
    border-collapse: collapse;
  }
  .md :global(:is(th, td)) {
    padding: var(--s-1) var(--s-3);
    border: 1px solid var(--border);
  }
  .md :global(th) {
    font-weight: var(--fw-semibold);
    text-align: left;
    background: var(--md-th-bg);
    border-bottom: 2px solid color-mix(in oklab, var(--accent) 35%, var(--border));
  }
  .md :global(tr:nth-child(2n) td) {
    background: var(--md-stripe-bg);
  }
  .md :global(hr) {
    height: 2px;
    margin: var(--s-6) 0;
    background: linear-gradient(90deg, color-mix(in oklab, var(--md-h2) 45%, var(--border)), var(--border) 60%);
    border: none;
  }
  .md :global(img) {
    max-width: 100%;
  }
  /* An image not loaded shows its alt text in a dashed box. */
  .md :global(img[data-src]:not([src])) {
    display: inline-block;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-sm);
  }
  .md :global(.mermaid-block) {
    margin: 0 0 var(--s-4);
    padding: var(--s-3);
    overflow: auto;
    text-align: center;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .md :global(.mermaid-block svg) {
    max-width: 100%;
    height: auto;
  }
  .md :global(.mermaid-error) {
    margin-top: calc(-1 * var(--s-3));
    font-size: var(--fs-sm);
    color: var(--err);
  }
  .md :global(del) {
    color: var(--fg-2);
  }
</style>
