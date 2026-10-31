<script lang="ts">
  import { renderMarkdown } from './render'

  // A Markdown file rendered (decision 84): the editor's text, so what-if
  // edits show. The app's routes live in the URL hash, so no link in here
  // may navigate the page: #heading scrolls within the preview, web links
  // open in a new window, links to files do nothing yet.
  interface Props {
    text: string
  }

  let { text }: Props = $props()

  let el = $state<HTMLDivElement>()
  const html = $derived(renderMarkdown(text))

  function onclick(e: MouseEvent) {
    const a = (e.target as Element).closest('a')
    if (!a || !el?.contains(a)) return
    e.preventDefault()
    const href = a.getAttribute('href') ?? ''
    if (href.startsWith('#')) {
      const id = decodeURIComponent(href.slice(1))
      el.querySelector(`[id="user-content-${CSS.escape(id)}"]`)?.scrollIntoView({ block: 'start' })
    } else if (/^(https?|mailto):/i.test(href)) {
      window.open(href, '_blank', 'noopener,noreferrer')
    }
  }
</script>

<!-- Clicks come from the links inside, which take keys themselves. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="preview" bind:this={el} {onclick}>
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
