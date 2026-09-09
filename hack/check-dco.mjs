#!/usr/bin/env node

import { execFileSync } from 'node:child_process'

const [base = process.env.DCO_BASE || 'main', head = process.env.DCO_HEAD || 'HEAD'] =
  process.argv.slice(2)
const signer = process.env.DCO_SIGNER || 'JeryGmaker <realgjt@163.com>'

function git(...args) {
  return execFileSync('git', args, { encoding: 'utf8' }).trim()
}

const commits = git('rev-list', `${base}..${head}`)
  .split(/\r?\n/)
  .filter(Boolean)

const missing = []
for (const commit of commits) {
  const body = git('show', '-s', '--format=%B', commit)
  const subject = git('show', '-s', '--format=%h %s', commit)
  if (!body.includes(`Signed-off-by: ${signer}`)) {
    missing.push(subject)
  }
}

console.log(`DCO range: ${base}..${head}`)
console.log(`Commits checked: ${commits.length}`)
console.log(`Signer: ${signer}`)
console.log(`Missing DCO: ${missing.length}`)
for (const subject of missing) console.log(`- ${subject}`)

if (missing.length > 0) process.exitCode = 1
