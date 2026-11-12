// The side panel of a lens view (Argo, pipelines, Compose, Ansible): a
// column of sections, each a heading and what is under it. This action
// gives every such panel the same two behaviours without each view
// writing them (3.10.2):
//
//  - a section folds: its heading is a toggle (click, Enter or Space),
//    and what is folded is remembered per view and heading (a display
//    preference);
//  - Problems comes first when there are any (it used to be last, below
//    everything else); the styles are in global.css (.lens-side).
//
// It works on the markup the views already have (section > h3), and
// keeps working as Svelte adds and removes sections.

const PREFIX = 'codec:folded:'

function load(key: string): Set<string> {
  try {
    return new Set(JSON.parse(localStorage.getItem(PREFIX + key) ?? '[]') as string[])
  } catch {
    return new Set()
  }
}

function save(key: string, folded: Set<string>) {
  try {
    localStorage.setItem(PREFIX + key, JSON.stringify([...folded]))
  } catch {
    /* persistence is optional */
  }
}

/** A heading's own words, without the count beside them. */
function nameOf(h: HTMLElement): string {
  let text = ''
  for (const n of h.childNodes) if (n.nodeType === Node.TEXT_NODE) text += n.textContent
  return text.trim().replace(/\s+/g, ' ')
}

export function foldSections(node: HTMLElement, key: string) {
  const folded = load(key)
  node.classList.add('lens-side')

  function apply() {
    for (const section of node.querySelectorAll<HTMLElement>(':scope > section')) {
      const h = section.querySelector<HTMLElement>(':scope > h3')
      if (!h) continue
      const name = nameOf(h)
      if (!name) continue
      const isFolded = folded.has(name)
      if (h.getAttribute('role') !== 'button') {
        h.setAttribute('role', 'button')
        h.tabIndex = 0
        h.classList.add('fold-toggle')
      }
      h.dataset.fold = name
      if (h.getAttribute('aria-expanded') !== String(!isFolded)) h.setAttribute('aria-expanded', String(!isFolded))
      section.toggleAttribute('data-folded', isFolded)
      // Problems first, when it has any (its heading then has a count).
      section.classList.toggle('problems-first', name === 'Problems' && !!h.querySelector('.count'))
    }
  }

  function toggle(h: HTMLElement) {
    const name = h.dataset.fold
    if (!name) return
    if (folded.has(name)) folded.delete(name)
    else folded.add(name)
    save(key, folded)
    apply()
  }
  const heading = (e: Event) => (e.target as HTMLElement).closest<HTMLElement>('h3.fold-toggle')
  function onclick(e: MouseEvent) {
    const h = heading(e)
    // A link or a button inside the heading keeps its own click.
    if (h && h.parentElement?.parentElement === node && !(e.target as HTMLElement).closest('a, button, input, select')) toggle(h)
  }
  function onkeydown(e: KeyboardEvent) {
    if (e.key !== 'Enter' && e.key !== ' ') return
    const h = e.target as HTMLElement
    if (h.matches?.('h3.fold-toggle') && h.parentElement?.parentElement === node) {
      e.preventDefault()
      toggle(h)
    }
  }

  apply()
  // Sections come and go as the analysis changes; the count beside
  // Problems appears and disappears.
  const mo = new MutationObserver(apply)
  mo.observe(node, { childList: true, subtree: true })
  node.addEventListener('click', onclick)
  node.addEventListener('keydown', onkeydown)
  return {
    destroy() {
      mo.disconnect()
      node.removeEventListener('click', onclick)
      node.removeEventListener('keydown', onkeydown)
    },
  }
}
