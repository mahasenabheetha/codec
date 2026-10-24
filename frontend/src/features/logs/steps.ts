// Matching an Argo run's status to its log, for the Steps tab and the
// Argo view's graph.

import type { ArgoRun, ArgoRunNode } from '../../lib/api/argo'
import type { LogStep, LogTally } from '../../lib/api/logs'
import { term } from './terms'

/** What the log has for a status node, and the query that shows it. */
export type Found = LogTally & { step: LogStep; filter: string }

const podOf = (source: string) => source.replace(/^pods?\//, '').split('/')[0]

/** A retry's attempt ("deploy(1)") or a loop's item ("deploy(0:eu)"). */
export const attempt = (n: ArgoRunNode) => /\(\d+(:[^)]*)?\)$/.test(n.displayName)

/** A pod node's own pod (exact, so a retry's attempts each get theirs);
 *  else the step of that name or template, where an attempt leaves the
 *  log's "deploy" to the retry node above it rather than each claiming it. */
export function findInLog(steps: LogStep[], n: ArgoRunNode): Found | undefined {
  if (n.type === 'Pod' && n.pod) {
    for (const st of steps) {
      const p = st.podStats?.find((ps) => podOf(ps.name) === n.pod)
      if (p) return { ...p, step: st, filter: term('source', p.name) }
    }
  }
  if ((n.type !== 'Pod' && n.type !== 'Retry') || attempt(n)) return undefined
  const st = steps.find((st) => st.name === n.displayName || st.name === n.template)
  return st && { ...st, step: st, filter: term('step', st.name) }
}

/** A step or task of a workflow, as the run went: its nodes in the run
 *  (a retry's attempts and a loop's items roll up into it). */
export interface RunOf {
  phase: string
  from?: string
  to?: string
  exitCode?: string
  message?: string
  nodes: ArgoRunNode[]
}

const failed = (p = '') => p === 'Failed' || p === 'Error'
const running = (p = '') => p === 'Running' || p === 'Pending'

/** The run's nodes for a step or task named name (the workflow's own
 *  names, as the Argo view shows them), rolled up; undefined when the
 *  run has none. */
export function runOf(run: ArgoRun, name: string): RunOf | undefined {
  const nodes = run.nodes.filter((n) => n.displayName === name || n.displayName.replace(/\(\d+(:[^)]*)?\)$/, '') === name)
  if (!nodes.length) return undefined
  const top = nodes.find((n) => !attempt(n)) ?? nodes[nodes.length - 1]
  const phases = nodes.map((n) => n.phase ?? '')
  const phase = phases.some(failed) && failed(top.phase) ? top.phase! : phases.some(running) ? 'Running' : top.phase || phases.find(Boolean) || ''
  const times = nodes.flatMap((n) => [n.startedAt, n.finishedAt]).filter(Boolean) as string[]
  times.sort()
  const last = [...nodes].reverse().find((n) => n.type === 'Pod')
  return {
    phase,
    from: times[0],
    to: phases.some(running) ? undefined : times[times.length - 1],
    exitCode: last?.exitCode,
    message: (failed(phase) && (last?.message || top.message)) || undefined,
    nodes,
  }
}

/** How many of the names the run has: to pick the run that fits a
 *  workflow best. */
export const runFit = (run: ArgoRun, names: string[]) => names.filter((nm) => runOf(run, nm)).length
