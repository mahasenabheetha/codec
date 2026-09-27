import { request } from './client'

export interface WorkspaceInfo {
  open: boolean
  root?: string
  name?: string
  watch?: 'native' | 'poll'
  recent: string[]
  sep: string
  /** The built-in sample workspace is open. */
  sample?: boolean
}

export interface FileEntry {
  path: string // slash-separated, relative to the root
  size: number
  lang: string // yaml, json, markdown, shell, …, text
  type?: string // provider id for YAML files, once classified
}

export interface FileTree {
  root: string
  files: FileEntry[]
  truncated?: boolean
}

export interface FileContent extends FileEntry {
  root: string
  text: string
  modTime: number // Unix ms
  bom?: boolean
  crlf?: boolean
}

export interface DirEntry {
  name: string
  path: string
  repo?: boolean
}

export interface DirListing {
  path: string
  parent?: string
  dirs: DirEntry[]
  home?: string
  roots?: string[]
  sep: string
}

export interface FileChange {
  op: 'added' | 'changed' | 'removed'
  path: string
}

export const getWorkspace = () => request<WorkspaceInfo>('GET', '/api/v2/workspace')

export const openWorkspace = (path: string) => request<WorkspaceInfo>('POST', '/api/v2/workspace/open', { path })

/** Writes the built-in sample repository to codec's cache folder and opens it. */
export const openSampleWorkspace = () => request<WorkspaceInfo>('POST', '/api/v2/workspace/sample')

export const listDirs = (path = '') =>
  request<DirListing>('GET', '/api/v2/fs/dirs' + (path ? '?path=' + encodeURIComponent(path) : ''))

export const getTree = () => request<FileTree>('GET', '/api/v2/files/tree')

export const getContent = (path: string) =>
  request<FileContent>('GET', '/api/v2/files/content?path=' + encodeURIComponent(path))
