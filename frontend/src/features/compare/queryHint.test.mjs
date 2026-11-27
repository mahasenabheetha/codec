// Query's hint for what it found: run with `npm test` (Node's own test
// runner; no dependencies).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { queryHint, unpinned } from './queryHint.ts'

test('which images a pull can change', () => {
  assert.equal(unpinned('nginx'), true)
  assert.equal(unpinned('bitnami/kubectl:latest'), true)
  assert.equal(unpinned('registry.local:5000/acme/web'), true) // the port is not a tag
  assert.equal(unpinned('ghcr.io/acme/shop:1.27'), false)
  assert.equal(unpinned('acme/web@sha256:' + 'a'.repeat(64)), false)
})

test('a hint for images under an image key or asked for', () => {
  const hits = [
    { path: '.spec.template.spec.containers[0].image', value: 'ghcr.io/acme/shop:1.27' },
    { path: '.spec.template.spec.containers[1].image', value: '"bitnami/kubectl:latest"' },
    { path: '.spec.jobTemplate.spec.template.spec.containers[0].image', value: 'acme/admin-ui' },
  ]
  assert.equal(queryHint('.spec', hits), "2 images use latest or no tag. Pin a tag so a pull can't change them.")
  assert.equal(queryHint('.. | .image? | strings', [{ value: 'nginx' }]), "1 image uses latest or no tag. Pin a tag so a pull can't change it.")
})

test('no hint for other values', () => {
  assert.equal(queryHint('.metadata.name', [{ path: '.metadata.name', value: 'web' }]), '')
  assert.equal(queryHint('.spec.replicas', [{ path: '.spec.replicas', value: '3' }]), '')
  assert.equal(queryHint('.. | .image?', [{ value: 'name: web\nimage: nginx' }]), '')
  assert.equal(queryHint('.image', []), '')
})
