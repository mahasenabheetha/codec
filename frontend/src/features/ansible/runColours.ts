// Colours a playbook map by what a loaded log run did: each task of the
// run is placed on the node it belongs to, and a node takes the worst
// status among its tasks; nodes no task reached never ran. Generic
// matching, no playbook evaluation:
//   - a task's "task path:" (-vv) ends with a task file's path, or lies
//     inside a role's folder;
//   - else its role prefix ("nginx : …") names a role node;
//   - handlers match handler nodes by name;
//   - anything else belongs to its play.
// A play node sums up every task of its play.

import type { LogRun, LogTask } from '../../lib/api/ansiblelog'
import type { PlaybookGraph, PlaybookNode } from '../../lib/api/ansible'

export type RunColour = 'failed' | 'changed' | 'ok' | 'rescued' | 'never'

const rank: Record<RunColour, number> = { never: 0, ok: 1, changed: 2, rescued: 3, failed: 4 }

function colourOf(t: LogTask): RunColour {
  switch (t.status) {
    case 'failed':
    case 'unreachable':
    case 'unfinished':
      return 'failed'
    case 'rescued':
      return 'rescued'
    case 'changed':
      return 'changed'
  }
  return 'ok'
}

const worse = (a: RunColour | undefined, b: RunColour): RunColour => (!a || rank[b] > rank[a] ? b : a)

/** Does a log's absolute task path point into a workspace path? */
function within(logPath: string, file: string): boolean {
  const p = '/' + logPath.replace(/\\/g, '/')
  const f = '/' + file.replace(/^\/+/, '')
  return p.endsWith(f) || p.includes(f + '/')
}

export function runColours(graph: PlaybookGraph, run: LogRun): Map<string, RunColour> {
  const out = new Map<string, RunColour>()
  const files = graph.nodes.filter((n) => n.kind === 'tasks' && n.file)
  // Longer paths are more specific: a role's task file beats the role.
  const roles = graph.nodes.filter((n) => n.kind === 'role')
  const plays = graph.nodes.filter((n) => n.kind === 'play')

  function nodeFor(t: LogTask, play: string): PlaybookNode | undefined {
    if (t.handler) {
      const h = graph.nodes.find((n) => n.kind === 'handler' && n.label === t.name)
      if (h) return h
    }
    if (t.path) {
      const file = files.filter((n) => within(t.path!, n.file!)).sort((a, b) => b.file!.length - a.file!.length)[0]
      if (file) return file
      const role = roles.find((n) => n.file && within(t.path!, n.file))
      if (role) return role
    }
    if (t.role) {
      const role = roles.find((n) => n.role === t.role)
      if (role) return role
    }
    return plays.find((n) => n.play === play) ?? (plays.length === 1 ? plays[0] : undefined)
  }

  for (const p of run.plays) {
    const playNode = plays.find((n) => n.play === p.name) ?? (plays.length === 1 ? plays[0] : undefined)
    for (const t of p.tasks) {
      const c = colourOf(t)
      const n = nodeFor(t, p.name)
      if (n) out.set(n.id, worse(out.get(n.id), c))
      if (playNode) out.set(playNode.id, worse(out.get(playNode.id), c))
    }
  }
  for (const n of graph.nodes) if (!out.has(n.id)) out.set(n.id, 'never')
  return out
}
