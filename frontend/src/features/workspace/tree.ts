// Turns the flat file list from the API into a folder tree, and the
// tree into the visible rows of the explorer.

import type { FileEntry } from '../../lib/api/workspace'

export interface TreeNode {
  name: string // display name; "a/b" for compacted single-child folders
  path: string // full slash path ("" for the root)
  file?: FileEntry // set for files
  children: TreeNode[] // folders first, then files
}

export interface TreeRow {
  node: TreeNode
  depth: number
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

/** Build a sorted tree. Folders with a single subfolder and nothing
 *  else are compacted into one row ("charts/app"), like VS Code. */
export function buildTree(files: FileEntry[]): TreeNode {
  const root: TreeNode = { name: '', path: '', children: [] }
  const dirs = new Map<string, TreeNode>([['', root]])

  function dir(path: string): TreeNode {
    let node = dirs.get(path)
    if (node) return node
    const i = path.lastIndexOf('/')
    const parent = dir(i < 0 ? '' : path.slice(0, i))
    node = { name: path.slice(i + 1), path, children: [] }
    parent.children.push(node)
    dirs.set(path, node)
    return node
  }

  for (const f of files) {
    const i = f.path.lastIndexOf('/')
    dir(i < 0 ? '' : f.path.slice(0, i)).children.push({ name: f.path.slice(i + 1), path: f.path, file: f, children: [] })
  }
  sortAndCompact(root)
  return root
}

function sortAndCompact(node: TreeNode) {
  node.children.sort((a, b) => {
    if (!a.file !== !b.file) return a.file ? 1 : -1
    return collator.compare(a.name, b.name)
  })
  for (let i = 0; i < node.children.length; i++) {
    let child = node.children[i]
    while (!child.file && child.children.length === 1 && !child.children[0].file) {
      const only = child.children[0]
      child = { ...only, name: child.name + '/' + only.name }
    }
    node.children[i] = child
    if (!child.file) sortAndCompact(child)
  }
}

/** Visible rows: children of expanded folders only (all of them when
 *  expandAll is set, e.g. while a filter is active). */
export function flatten(root: TreeNode, expanded: Set<string>, expandAll = false): TreeRow[] {
  const rows: TreeRow[] = []
  const walk = (node: TreeNode, depth: number) => {
    for (const child of node.children) {
      rows.push({ node: child, depth })
      if (!child.file && (expandAll || expanded.has(child.path))) walk(child, depth + 1)
    }
  }
  walk(root, 0)
  return rows
}

/** Every folder path above a file, e.g. a, a/b for a/b/c.yaml. */
export function ancestors(path: string): string[] {
  const parts = path.split('/')
  return parts.slice(0, -1).map((_, i) => parts.slice(0, i + 1).join('/'))
}
