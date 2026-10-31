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
  import { openLink } from '../../lib/platform'
  import { layout } from '../../lib/stores/layout.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import { linkTarget, type Target } from './links'
  import { renderMarkdown, sanitizeSVG } from './render'

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
      <ImageOff size={14} strokeWidth={1.75} />
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
  .md {
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
    border-bottom: 1px solid var(--border);
  }
  .md :global(h2) {
    font-size: var(--fs-2xl);
    padding-bottom: var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  .md :global(h3) {
    font-size: var(--fs-xl);
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
    margin: 0 var(--s-2) 0 -1.4em;
    vertical-align: middle;
  }
  .md :global(blockquote) {
    padding: 0 var(--s-4);
    color: var(--fg-1);
    border-left: 3px solid var(--border-strong);
  }
  .md :global(code) {
    padding: 0.15em 0.4em;
    font-family: var(--font-mono);
    font-size: 0.86em;
    background: var(--bg-3);
    border-radius: var(--r-sm);
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
    background: var(--bg-1);
  }
  .md :global(tr:nth-child(2n) td) {
    background: color-mix(in srgb, var(--bg-1) 50%, transparent);
  }
  .md :global(hr) {
    height: 2px;
    margin: var(--s-6) 0;
    background: var(--border);
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
  .md :global(del) {
    color: var(--fg-2);
  }
</style>
