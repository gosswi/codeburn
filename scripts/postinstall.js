#!/usr/bin/env node
// postinstall.js
// Checks whether a native Go binary for codeburn is available on PATH.
// If found, prints a notice so users know they can use it directly.
// Never fails - this is informational only.

import { spawnSync } from 'node:child_process'

function findGoBinary() {
  // spawnSync with fixed args: no user input, no injection risk.
  const result = spawnSync('which', ['codeburn'], { encoding: 'utf8' })
  if (result.status === 0 && result.stdout) {
    const p = result.stdout.trim()
    if (p && !p.includes('node_modules')) {
      return p
    }
  }
  return null
}

const go = findGoBinary()
if (go) {
  console.log(`\ncodeburn: Go binary found at ${go}`)
  console.log('The Go binary starts faster (~5ms vs ~100ms) and needs no Node.js runtime.')
  console.log('You can use it directly or via: npm exec codeburn\n')
}
