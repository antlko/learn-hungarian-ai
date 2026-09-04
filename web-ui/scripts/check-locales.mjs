#!/usr/bin/env node
/**
 * ADR-004: both locales are complete, always. A key in one file and not the
 * other is an incomplete change, not a follow-up. This is the same walk the CI
 * job runs; it lives here so `npm run lint` catches it before a push does.
 */
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const load = (name) => JSON.parse(readFileSync(join(here, '..', 'src', 'locales', `${name}.json`), 'utf8'))

function keys(obj, prefix = '', out = new Set()) {
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k
    out.add(path)
    if (v && typeof v === 'object' && !Array.isArray(v)) keys(v, path, out)
    else if (typeof v !== 'string') {
      console.error(`locale value at '${path}' is ${v === null ? 'null' : typeof v}, expected a string`)
      process.exitCode = 1
    }
  }
  return out
}

const ru = keys(load('ru'))
const en = keys(load('en'))
let missing = 0
for (const [have, lack, name] of [
  [ru, en, 'en'],
  [en, ru, 'ru'],
]) {
  for (const k of [...have].filter((k) => !lack.has(k)).sort()) {
    console.error(`locale key '${k}' is missing from ${name}.json`)
    missing++
  }
}
if (missing) process.exitCode = 1
else console.log(`locales in sync: ${ru.size} keys in ru and en`)
