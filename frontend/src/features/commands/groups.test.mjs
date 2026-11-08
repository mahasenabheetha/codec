// The tools as lists show them: npm test.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { inScope, scopeItems, sections } from './groups.ts'

const tools = [
  { name: 'tar', summary: '', group: 'linux', category: 'archives' },
  { name: 'ls', summary: '', group: 'linux', category: 'files' },
  { name: 'curl', summary: '', group: 'linux', category: 'network' },
  { name: 'cp', summary: '', group: 'linux', category: 'files' },
  { name: 'kubectl', summary: '', group: 'kubernetes' },
  { name: 'helm', summary: '', group: 'kubernetes' },
  { name: 'git', summary: '', group: 'git' },
  { name: 'docker', summary: '', group: 'containers' },
]

test('groups in their order, Linux by category', () => {
  const s = sections(tools)
  assert.deepEqual(
    s.map((x) => x.label),
    ['Git', 'Kubernetes', 'Containers', 'Linux'],
  )
  const linux = s[3]
  assert.equal(linux.count, 4)
  assert.deepEqual(
    linux.categories.map((c) => [c.label, c.tools.map((t) => t.name)]),
    [
      ['Files and folders', ['cp', 'ls']],
      ['Archives', ['tar']],
      ['Network', ['curl']],
    ],
  )
  assert.deepEqual(s[1].categories, [{ category: '', label: '', tools: [tools[5], tools[4]] }])
})

test('the Tool menu', () => {
  const items = scopeItems(tools)
  const v = items.map((i) => i.value)
  assert.equal(v[0], '')
  assert.ok(v.includes('group:linux') && v.includes('group:kubernetes') && v.includes('category:files'))
  assert.ok(!v.includes('group:git'), 'a group of one tool has no "All"')
  const files = items.find((i) => i.value === 'category:files')
  assert.equal(files.selectedLabel, 'Linux: Files and folders')
  assert.equal(items.find((i) => i.value === 'ls').group, 'Linux · Files and folders')
  assert.equal(items.find((i) => i.value === 'git').group, 'Git')
})

test('scope as the server reads it', () => {
  const [tar, ls, , , kubectl] = tools
  assert.ok(inScope(tar, '') && inScope(tar, 'tar') && inScope(tar, 'group:linux') && inScope(tar, 'category:archives'))
  assert.ok(!inScope(ls, 'category:archives') && !inScope(kubectl, 'group:linux') && !inScope(ls, 'tar'))
})
