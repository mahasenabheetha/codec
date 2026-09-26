// Workspace state shared by the explorer, file tabs, Go to file and the
// status bar: the open folder, its file list, and live change events.

import { SvelteMap, SvelteSet } from 'svelte/reactivity'
import { ApiError, eventStream } from '../../lib/api/client'
import {
  getTree,
  getWorkspace,
  openWorkspace,
  type FileChange,
  type FileEntry,
  type WorkspaceInfo,
} from '../../lib/api/workspace'
import { layout } from '../../lib/stores/layout.svelte'
import { persisted } from '../../lib/stores/persist.svelte'
import { routeFile } from '../../lib/stores/router.svelte'
import { toast } from '../../lib/stores/toast.svelte'
import { ancestors } from './tree'

// The folder the persisted file tabs and expanded folders belong to.
const lastRoot = persisted('workspaceRoot', '')
const expandedStore = persisted<string[]>('explorerExpanded', [])

class Workspace {
  info = $state<WorkspaceInfo | null>(null)
  files = $state.raw<FileEntry[]>([])
  truncated = $state(false)
  treeLoading = $state(false)
  treeError = $state<string | null>(null)
  /** The server restarted or went away; the page needs a reload. */
  disconnected = $state(false)

  dialogOpen = $state(false)
  quickOpen = $state(false)

  /** Folders open in the explorer. */
  expanded = new SvelteSet<string>(expandedStore.value)
  /** Bumped per path when the file changes on disk; file views reload. */
  versions = new SvelteMap<string, number>()
  /** Paths deleted on disk while open in a tab. */
  deleted = new SvelteSet<string>()
  /** Open files with what-if edits (tab dot). */
  dirty = new SvelteSet<string>()
  /** Recently opened files this session, newest first (Go to file). */
  recentFiles = $state<string[]>([])

  byPath = $derived(new Map(this.files.map((f) => [f.path, f])))

  private events: EventSource | null = null
  private reloadTimer: ReturnType<typeof setTimeout> | undefined

  constructor() {
    $effect.root(() => {
      $effect(() => {
        expandedStore.value = [...this.expanded]
      })
    })
  }

  /** Load state at startup; reopen the last folder after a server restart. */
  async init() {
    try {
      this.info = await getWorkspace()
    } catch {
      return // server unreachable: the status bar shows it on the next call
    }
    this.connect()
    if (!this.info.open && lastRoot.value && this.info.recent.includes(lastRoot.value)) {
      try {
        this.info = await openWorkspace(lastRoot.value)
      } catch {
        // Moved or deleted since; the user picks another folder.
      }
    }
    this.rootChanged()
    if (this.info.open) await this.loadTree()
  }

  async open(path: string) {
    this.info = await openWorkspace(path)
    this.dialogOpen = false
    this.rootChanged()
    await this.loadTree()
  }

  async loadTree() {
    if (!this.info?.open) {
      this.files = []
      return
    }
    this.treeLoading = true
    try {
      const t = await getTree()
      if (t.root !== this.info.root) return // a newer folder was opened meanwhile
      this.files = t.files
      this.truncated = !!t.truncated
      this.treeError = null
    } catch (e) {
      this.treeError = e instanceof Error ? e.message : String(e)
      if (e instanceof ApiError && e.status === 401) this.disconnected = true
    } finally {
      this.treeLoading = false
    }
  }

  /** Expand the folders above path so the explorer shows it. */
  reveal(path: string) {
    for (const a of ancestors(path)) this.expanded.add(a)
  }

  noteOpened(path: string) {
    this.recentFiles = [path, ...this.recentFiles.filter((p) => p !== path)].slice(0, 20)
  }

  // A different folder than the tabs and expanded folders were saved
  // for: they no longer mean anything, so start clean.
  private rootChanged() {
    const root = this.info?.open ? this.info.root! : ''
    if (root === lastRoot.value) return
    const first = lastRoot.value === ''
    lastRoot.value = root
    // Nothing was saved for another folder yet (first visit): keep a
    // file tab the URL opened, e.g. a shared link.
    if (first) return
    layout.closeWhere((r) => routeFile(r) !== null)
    this.expanded.clear()
    this.versions.clear()
    this.deleted.clear()
    this.recentFiles = []
  }

  private connect() {
    this.events?.close()
    const es = eventStream('/api/v2/events')
    this.events = es
    let opened = false

    es.addEventListener('open', () => {
      // After a reconnect, events may have been missed: resync.
      if (opened) this.loadTree()
      opened = true
      this.disconnected = false
    })
    es.addEventListener('error', () => {
      // EventSource retries by itself; CLOSED means it gave up (e.g. a
      // restarted server rejects the old token).
      if (es.readyState === EventSource.CLOSED) this.disconnected = true
    })
    es.addEventListener('workspace', (e) => {
      this.info = JSON.parse((e as MessageEvent).data)
      this.rootChanged()
      this.loadTree()
    })
    es.addEventListener('tree', () => this.scheduleReload())
    es.addEventListener('files', (e) => {
      const { root, changes } = JSON.parse((e as MessageEvent).data) as { root: string; changes: FileChange[] }
      if (root !== this.info?.root) return
      for (const c of changes) {
        if (c.op === 'removed') this.deleted.add(c.path)
        else this.deleted.delete(c.path)
        this.versions.set(c.path, (this.versions.get(c.path) ?? 0) + 1)
      }
      this.scheduleReload()
    })
  }

  // Coalesce bursts of events into one tree request.
  private scheduleReload() {
    clearTimeout(this.reloadTimer)
    this.reloadTimer = setTimeout(() => this.loadTree(), 50)
  }
}

export const workspace = new Workspace()

/** Open a folder, reporting failures as a toast. Returns success. */
export async function openFolder(path: string): Promise<boolean> {
  try {
    await workspace.open(path)
    return true
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'err')
    return false
  }
}
