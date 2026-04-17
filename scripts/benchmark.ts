#!/usr/bin/env tsx
/**
 * Benchmark: installed codeburn (npm v0.5.0) vs local Phase 2 build
 *
 * Usage:
 *   npx tsx scripts/benchmark.ts [--runs N] [--out path]
 *
 * Outputs a self-contained HTML report.
 */

import { spawnSync } from 'child_process'
import { writeFileSync, rmSync, existsSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'
import { homedir, cpus, totalmem } from 'os'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const ROOT = join(__dirname, '..')
const LOCAL_CLI = join(ROOT, 'dist', 'cli.js')
const INSTALLED_BIN = '/opt/homebrew/bin/codeburn'
const CACHE_DB = join(homedir(), '.cache', 'codeburn', 'session-cache.db')
const TIME_BIN = '/usr/bin/time'

const argRuns = process.argv.indexOf('--runs')
const argOut = process.argv.indexOf('--out')
const RUNS = argRuns !== -1 ? parseInt(process.argv[argRuns + 1], 10) : 7
const OUTPUT = argOut !== -1 ? process.argv[argOut + 1] : join(ROOT, 'benchmark-results.html')

const COMMANDS = [
  {
    name: 'status --format json',
    args: ['status', '--format', 'json'],
    desc: 'Compact JSON — most common programmatic use',
  },
  {
    name: 'status --format menubar',
    args: ['status', '--format', 'menubar'],
    desc: 'macOS menu bar widget',
  },
  {
    name: 'status --format terminal',
    args: ['status', '--format', 'terminal'],
    desc: 'Human-readable terminal output',
  },
]

// ---------------------------------------------------------------------------
// Sampling
// ---------------------------------------------------------------------------

interface Sample { ms: number; rssKB: number }

function clearCache(): void {
  if (existsSync(CACHE_DB)) rmSync(CACHE_DB)
}

function sample(bin: string, cmdArgs: string[]): Sample {
  const result = spawnSync(TIME_BIN, ['-l', bin, ...cmdArgs], {
    stdio: ['ignore', 'ignore', 'pipe'],
    encoding: 'utf8',
  })
  const stderr = result.stderr ?? ''
  const realMatch = stderr.match(/([\d.]+)\s+real/)
  const rssMatch = stderr.match(/(\d+)\s+maximum resident set size/)
  return {
    ms: realMatch ? Math.round(parseFloat(realMatch[1]) * 1000) : 0,
    rssKB: rssMatch ? Math.round(parseInt(rssMatch[1], 10) / 1024) : 0,
  }
}

function aggregate(samples: Sample[]) {
  const sorted = [...samples].sort((a, b) => a.ms - b.ms)
  const mid = Math.floor(sorted.length / 2)
  return {
    median: sorted.length % 2 === 0
      ? Math.round((sorted[mid - 1].ms + sorted[mid].ms) / 2)
      : sorted[mid].ms,
    min: sorted[0].ms,
    max: sorted[sorted.length - 1].ms,
    rssKB: sorted[mid].rssKB,
    all: samples.map(s => s.ms),
  }
}

// ---------------------------------------------------------------------------
// Benchmark loop
// ---------------------------------------------------------------------------

interface CommandResult {
  name: string
  desc: string
  installed: ReturnType<typeof aggregate>
  localCold: ReturnType<typeof aggregate>
  localWarm: ReturnType<typeof aggregate>
}

function log(msg: string) { process.stdout.write(msg + '\n') }
function logInline(msg: string) { process.stdout.write('\r\x1b[K' + msg) }

log('\nCodeBurn Benchmark')
log('==================')
log(`Runs per variant: ${RUNS}`)
log(`Output: ${OUTPUT}\n`)

const results: CommandResult[] = []

for (const cmd of COMMANDS) {
  log(`\n[${cmd.name}]`)

  // --- Installed ---
  spawnSync(INSTALLED_BIN, cmd.args, { stdio: 'ignore' }) // warmup
  const installedSamples: Sample[] = []
  for (let i = 0; i < RUNS; i++) {
    logInline(`  installed   run ${i + 1}/${RUNS}`)
    installedSamples.push(sample(INSTALLED_BIN, cmd.args))
  }
  log(`\r\x1b[K  installed   done  median=${aggregate(installedSamples).median}ms`)

  // --- Local cold (clear cache before each run) ---
  clearCache()
  const localColdSamples: Sample[] = []
  for (let i = 0; i < RUNS; i++) {
    clearCache()
    logInline(`  local-cold  run ${i + 1}/${RUNS}`)
    localColdSamples.push(sample('node', [LOCAL_CLI, ...cmd.args]))
  }
  log(`\r\x1b[K  local-cold  done  median=${aggregate(localColdSamples).median}ms`)

  // --- Local warm (SQLite cache pre-populated, kept warm across all runs) ---
  clearCache()
  spawnSync('node', [LOCAL_CLI, ...cmd.args], { stdio: 'ignore' }) // populate cache
  spawnSync('node', [LOCAL_CLI, ...cmd.args], { stdio: 'ignore' }) // second pass
  const localWarmSamples: Sample[] = []
  for (let i = 0; i < RUNS; i++) {
    logInline(`  local-warm  run ${i + 1}/${RUNS}`)
    localWarmSamples.push(sample('node', [LOCAL_CLI, ...cmd.args]))
  }
  log(`\r\x1b[K  local-warm  done  median=${aggregate(localWarmSamples).median}ms`)

  results.push({
    name: cmd.name,
    desc: cmd.desc,
    installed: aggregate(installedSamples),
    localCold: aggregate(localColdSamples),
    localWarm: aggregate(localWarmSamples),
  })
}

// Leave cache in warm state
clearCache()
spawnSync('node', [LOCAL_CLI, 'status', '--format', 'json'], { stdio: 'ignore' })

// ---------------------------------------------------------------------------
// System info
// ---------------------------------------------------------------------------

const nodeVersion = process.version
const installedVersion = spawnSync(INSTALLED_BIN, ['--version'], { encoding: 'utf8' }).stdout.trim()
const localVersion = spawnSync('node', [LOCAL_CLI, '--version'], { encoding: 'utf8' }).stdout.trim()
const cpuModel = cpus()[0]?.model ?? 'unknown'
const cpuCount = cpus().length
const totalRamGB = (totalmem() / 1024 / 1024 / 1024).toFixed(1)
const timestamp = new Date().toISOString()

// ---------------------------------------------------------------------------
// HTML generation
// ---------------------------------------------------------------------------

function speedupBadge(factor: number): string {
  const color = factor >= 3 ? '#3fb950' : factor >= 2 ? '#58a6ff' : factor >= 1.3 ? '#d29922' : '#f85149'
  return `<span class="badge" style="background:${color}20;color:${color};border-color:${color}40">${factor.toFixed(1)}x faster</span>`
}

function rssBadge(installedKB: number, localKB: number): string {
  const delta = installedKB - localKB
  const pct = Math.round((delta / installedKB) * 100)
  if (pct > 0) {
    return `<span class="badge" style="background:#3fb95020;color:#3fb950;border-color:#3fb95040">-${pct}% RSS</span>`
  }
  return `<span class="badge" style="background:#f8514920;color:#f85149;border-color:#f8514940">+${Math.abs(pct)}% RSS</span>`
}

const chartDatasets = results.map(r => ({
  name: r.name,
  desc: r.desc,
  installed: r.installed.median,
  localCold: r.localCold.median,
  localWarm: r.localWarm.median,
  installedRssKB: r.installed.rssKB,
  localColdRssKB: r.localCold.rssKB,
  localWarmRssKB: r.localWarm.rssKB,
  warmSpeedup: parseFloat((r.installed.median / r.localWarm.median).toFixed(2)),
  coldSpeedup: parseFloat((r.installed.median / r.localCold.median).toFixed(2)),
}))

const globalWarmSpeedup = (
  chartDatasets.reduce((s, d) => s + d.warmSpeedup, 0) / chartDatasets.length
).toFixed(1)

const summaryCards = chartDatasets.map(d => `
  <div class="summary-card">
    <div class="cmd-name">${d.name}</div>
    <div class="cmd-desc">${d.desc}</div>
    <div class="speedup-row">${speedupBadge(d.warmSpeedup)} warm cache</div>
    <div class="speedup-row">${speedupBadge(d.coldSpeedup)} cold start</div>
    <div class="timing-row">
      <span class="t-label">v0.5.0 (npm)</span>
      <span class="t-value installed">${d.installed}ms</span>
    </div>
    <div class="timing-row">
      <span class="t-label">Phase 2 (warm)</span>
      <span class="t-value local-warm">${d.localWarm}ms</span>
    </div>
  </div>
`).join('')

const commandSections = results.map((r, i) => {
  const d = chartDatasets[i]
  const maxMs = Math.max(r.installed.median, r.localCold.median, r.localWarm.median) * 1.08
  const maxRssKB = Math.max(r.installed.rssKB, r.localCold.rssKB, r.localWarm.rssKB) * 1.08

  function bar(value: number, max: number, cls: string, label: string, sub: string, unit: string) {
    const pct = Math.max(4, Math.round((value / max) * 100))
    return `
      <div class="bar-row">
        <div class="bar-label">
          <span>${label}</span>
          <span class="bar-sublabel">${sub}</span>
        </div>
        <div class="bar-track">
          <div class="bar-fill ${cls}" style="width:${pct}%">
            <span class="bar-value">${value}${unit}</span>
          </div>
        </div>
      </div>`
  }

  const maxDotMs = r.installed.max * 1.1
  const allRunDots = [
    { label: 'installed', runs: r.installed.all, cls: 'installed' },
    { label: 'cold', runs: r.localCold.all, cls: 'local-cold' },
    { label: 'warm', runs: r.localWarm.all, cls: 'local-warm' },
  ].map(v =>
    v.runs.map((ms, j) =>
      `<div class="dot ${v.cls}" style="left:${Math.min(96, Math.round((ms / maxDotMs) * 100))}%" title="${v.label} run ${j + 1}: ${ms}ms"></div>`
    ).join('')
  ).join('')

  const rawRows = Array.from({ length: RUNS }, (_, j) => `
    <tr>
      <td>${j + 1}</td>
      <td class="installed">${r.installed.all[j] ?? '-'}</td>
      <td class="local-cold">${r.localCold.all[j] ?? '-'}</td>
      <td class="local-warm">${r.localWarm.all[j] ?? '-'}</td>
    </tr>`).join('')

  return `
  <div class="cmd-section">
    <h2 class="cmd-title">
      <span class="cmd-mono">${r.name}</span>
      <span class="cmd-desc-inline">${r.desc}</span>
    </h2>

    <div class="charts-grid">
      <div class="chart-card">
        <div class="chart-title">Wall-clock time (median of ${RUNS} runs)</div>
        ${bar(r.installed.median, maxMs, 'installed', 'v0.5.0 (npm)', 'no SQLite cache', 'ms')}
        ${bar(r.localCold.median, maxMs, 'local-cold', 'Phase 2 (cold)', 'cache miss', 'ms')}
        ${bar(r.localWarm.median, maxMs, 'local-warm', 'Phase 2 (warm)', 'SQLite cache hit', 'ms')}
        <div class="scatter-section">
          <div class="scatter-title">All ${RUNS} runs per variant</div>
          <div class="scatter-track">${allRunDots}</div>
          <div class="scatter-legend">
            <span class="legend-dot installed"></span>installed &nbsp;
            <span class="legend-dot local-cold"></span>cold &nbsp;
            <span class="legend-dot local-warm"></span>warm
          </div>
        </div>
      </div>

      <div class="chart-card">
        <div class="chart-title">Peak RSS memory (median run)</div>
        ${bar(Math.round(r.installed.rssKB / 1024), maxRssKB / 1024, 'installed', 'v0.5.0 (npm)', 'no SQLite cache', 'MB')}
        ${bar(Math.round(r.localCold.rssKB / 1024), maxRssKB / 1024, 'local-cold', 'Phase 2 (cold)', 'cache miss', 'MB')}
        ${bar(Math.round(r.localWarm.rssKB / 1024), maxRssKB / 1024, 'local-warm', 'Phase 2 (warm)', 'SQLite cache hit', 'MB')}
        <div class="stat-row">
          <div class="stat">
            <div class="stat-label">RSS delta (warm)</div>
            <div class="stat-value">${rssBadge(r.installed.rssKB, r.localWarm.rssKB)}</div>
          </div>
          <div class="stat">
            <div class="stat-label">Speedup (warm)</div>
            <div class="stat-value">${speedupBadge(d.warmSpeedup)}</div>
          </div>
          <div class="stat">
            <div class="stat-label">Speedup (cold)</div>
            <div class="stat-value">${speedupBadge(d.coldSpeedup)}</div>
          </div>
        </div>
      </div>
    </div>

    <details class="raw-details">
      <summary>Raw samples (ms)</summary>
      <table class="raw-table">
        <thead>
          <tr>
            <th>Run</th>
            <th class="installed">v0.5.0 (ms)</th>
            <th class="local-cold">Cold (ms)</th>
            <th class="local-warm">Warm (ms)</th>
          </tr>
        </thead>
        <tbody>
          ${rawRows}
          <tr class="stat-row-table">
            <td>median</td>
            <td class="installed">${r.installed.median}</td>
            <td class="local-cold">${r.localCold.median}</td>
            <td class="local-warm">${r.localWarm.median}</td>
          </tr>
          <tr>
            <td>min</td>
            <td class="installed">${r.installed.min}</td>
            <td class="local-cold">${r.localCold.min}</td>
            <td class="local-warm">${r.localWarm.min}</td>
          </tr>
          <tr>
            <td>max</td>
            <td class="installed">${r.installed.max}</td>
            <td class="local-cold">${r.localCold.max}</td>
            <td class="local-warm">${r.localWarm.max}</td>
          </tr>
          <tr>
            <td>RSS (MB)</td>
            <td class="installed">${Math.round(r.installed.rssKB / 1024)}</td>
            <td class="local-cold">${Math.round(r.localCold.rssKB / 1024)}</td>
            <td class="local-warm">${Math.round(r.localWarm.rssKB / 1024)}</td>
          </tr>
        </tbody>
      </table>
    </details>
  </div>`
}).join('')

const html = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>CodeBurn Benchmark - v${installedVersion} vs Phase 2</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0 }

  :root {
    --bg:        #0d1117;
    --bg2:       #161b22;
    --bg3:       #21262d;
    --border:    #30363d;
    --text:      #e6edf3;
    --text2:     #8b949e;
    --installed: #f0883e;
    --cold:      #58a6ff;
    --warm:      #3fb950;
    --mono: 'JetBrains Mono', monospace;
    --sans: 'Inter', system-ui, sans-serif;
  }

  body {
    background: var(--bg);
    color: var(--text);
    font-family: var(--sans);
    font-size: 14px;
    line-height: 1.6;
    padding-bottom: 80px;
  }

  /* ---- Header ---- */
  .header {
    background: linear-gradient(135deg, #161b22 0%, #0d1117 55%, #0e1a0f 100%);
    border-bottom: 1px solid var(--border);
    padding: 52px 32px 44px;
    position: relative;
    overflow: hidden;
  }
  .header::before {
    content: '';
    position: absolute;
    top: -80px; right: -80px;
    width: 360px; height: 360px;
    background: radial-gradient(circle, rgba(63,185,80,0.07) 0%, transparent 70%);
    pointer-events: none;
  }
  .header::after {
    content: '';
    position: absolute;
    bottom: -60px; left: 60px;
    width: 240px; height: 240px;
    background: radial-gradient(circle, rgba(240,136,62,0.05) 0%, transparent 70%);
    pointer-events: none;
  }
  .header-inner { max-width: 960px; margin: 0 auto; position: relative; }

  .header h1 {
    font-size: 30px;
    font-weight: 700;
    letter-spacing: -0.6px;
    margin-bottom: 8px;
  }
  .header h1 .accent { color: var(--warm); }

  .header-sub {
    color: var(--text2);
    font-size: 15px;
    margin-bottom: 22px;
  }

  .version-pills { display: flex; gap: 10px; flex-wrap: wrap; margin-bottom: 24px; }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 5px 14px;
    border-radius: 20px;
    font-size: 12px;
    font-family: var(--mono);
    font-weight: 500;
    border: 1px solid;
  }
  .pill-installed { background: rgba(240,136,62,0.08); color: var(--installed); border-color: rgba(240,136,62,0.25); }
  .pill-local     { background: rgba(63,185,80,0.08);  color: var(--warm);      border-color: rgba(63,185,80,0.25);  }
  .pill-dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; flex-shrink: 0; }

  .headline-speedup {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    padding: 10px 20px;
    background: rgba(63,185,80,0.07);
    border: 1px solid rgba(63,185,80,0.2);
    border-radius: 10px;
    font-size: 15px;
    font-weight: 600;
    color: var(--warm);
  }
  .headline-speedup .big { font-size: 32px; font-weight: 700; letter-spacing: -1px; }

  /* ---- Content wrapper ---- */
  .content { max-width: 960px; margin: 0 auto; padding: 0 32px; }

  /* ---- System info ---- */
  .sysinfo {
    margin-top: 36px;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
  }
  .sysinfo-card {
    background: var(--bg2);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 16px;
  }
  .sysinfo-label { color: var(--text2); font-size: 10px; text-transform: uppercase; letter-spacing: 0.6px; margin-bottom: 5px; }
  .sysinfo-value { font-family: var(--mono); font-size: 12px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  /* ---- Section titles ---- */
  h2.section-title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text2);
    margin: 44px 0 16px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border);
  }

  /* ---- Summary cards ---- */
  .summary-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 14px;
  }
  .summary-card {
    background: var(--bg2);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 20px;
  }
  .cmd-name { font-family: var(--mono); font-size: 13px; font-weight: 600; margin-bottom: 4px; }
  .cmd-desc { color: var(--text2); font-size: 12px; margin-bottom: 14px; }
  .speedup-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
    font-size: 12px;
    color: var(--text2);
  }
  .timing-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 14px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
    font-size: 12px;
  }
  .t-label { color: var(--text2); }
  .t-value { font-family: var(--mono); font-weight: 700; }
  .t-value.installed  { color: var(--installed); }
  .t-value.local-warm { color: var(--warm); }

  /* ---- Badge ---- */
  .badge {
    display: inline-block;
    padding: 2px 9px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    border: 1px solid;
    font-family: var(--mono);
  }

  /* ---- Command sections ---- */
  .cmd-section { margin-bottom: 52px; }

  h2.cmd-title {
    font-size: 16px;
    font-weight: 600;
    margin: 40px 0 16px;
    display: flex;
    align-items: baseline;
    gap: 12px;
    flex-wrap: wrap;
  }
  .cmd-mono        { font-family: var(--mono); color: var(--text); }
  .cmd-desc-inline { font-size: 12px; color: var(--text2); font-weight: 400; }

  .charts-grid {
    display: grid;
    grid-template-columns: 3fr 2fr;
    gap: 16px;
  }
  @media (max-width: 680px) { .charts-grid { grid-template-columns: 1fr; } }

  .chart-card {
    background: var(--bg2);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 22px;
  }
  .chart-title {
    font-size: 11px;
    color: var(--text2);
    font-weight: 600;
    margin-bottom: 18px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  /* ---- Bars ---- */
  .bar-row { margin-bottom: 14px; }
  .bar-label {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
    color: var(--text2);
    margin-bottom: 5px;
  }
  .bar-sublabel { font-size: 10px; opacity: 0.65; }
  .bar-track {
    height: 30px;
    background: var(--bg3);
    border-radius: 5px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    border-radius: 5px;
    display: flex;
    align-items: center;
    padding: 0 10px;
    min-width: 52px;
  }
  .bar-fill.installed  { background: linear-gradient(90deg, rgba(240,136,62,0.75), rgba(240,136,62,0.35)); }
  .bar-fill.local-cold { background: linear-gradient(90deg, rgba(88,166,255,0.75), rgba(88,166,255,0.35)); }
  .bar-fill.local-warm { background: linear-gradient(90deg, rgba(63,185,80,0.80), rgba(63,185,80,0.40)); }
  .bar-value {
    font-family: var(--mono);
    font-size: 12px;
    font-weight: 700;
    color: #fff;
    white-space: nowrap;
  }

  /* ---- Scatter strip ---- */
  .scatter-section { margin-top: 22px; }
  .scatter-title { font-size: 11px; color: var(--text2); margin-bottom: 8px; }
  .scatter-track {
    height: 26px;
    background: var(--bg3);
    border-radius: 5px;
    position: relative;
  }
  .dot {
    position: absolute;
    top: 50%;
    width: 9px; height: 9px;
    border-radius: 50%;
    transform: translate(-50%, -50%);
    cursor: default;
    transition: transform 0.1s;
    border: 1.5px solid rgba(0,0,0,0.3);
  }
  .dot:hover { transform: translate(-50%, -50%) scale(1.6); z-index: 2; }
  .dot.installed  { background: var(--installed); }
  .dot.local-cold { background: var(--cold); }
  .dot.local-warm { background: var(--warm); }

  .scatter-legend {
    display: flex;
    gap: 6px;
    margin-top: 8px;
    font-size: 11px;
    color: var(--text2);
    align-items: center;
    flex-wrap: wrap;
  }
  .legend-dot {
    display: inline-block;
    width: 8px; height: 8px;
    border-radius: 50%;
    margin-right: 3px;
    flex-shrink: 0;
  }
  .legend-dot.installed  { background: var(--installed); }
  .legend-dot.local-cold { background: var(--cold); }
  .legend-dot.local-warm { background: var(--warm); }

  /* ---- Stat row (inside chart card) ---- */
  .stat-row {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
    margin-top: 22px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
  .stat { flex: 1; min-width: 70px; }
  .stat-label { font-size: 11px; color: var(--text2); margin-bottom: 6px; text-transform: uppercase; letter-spacing: 0.4px; }

  /* ---- Raw samples table ---- */
  .raw-details {
    margin-top: 12px;
    background: var(--bg2);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .raw-details summary {
    padding: 10px 18px;
    cursor: pointer;
    color: var(--text2);
    font-size: 12px;
    user-select: none;
    list-style: none;
  }
  .raw-details summary::-webkit-details-marker { display: none; }
  .raw-details summary::before { content: '+ '; }
  .raw-details[open] summary::before { content: '- '; }
  .raw-details summary:hover { color: var(--text); }

  .raw-table {
    width: 100%;
    border-collapse: collapse;
    font-family: var(--mono);
    font-size: 12px;
  }
  .raw-table th, .raw-table td {
    padding: 8px 18px;
    text-align: right;
    border-top: 1px solid var(--border);
  }
  .raw-table th { color: var(--text2); font-weight: 500; }
  .raw-table th:first-child, .raw-table td:first-child {
    text-align: left;
    color: var(--text2);
  }
  .raw-table .installed  { color: var(--installed); }
  .raw-table .local-cold { color: var(--cold); }
  .raw-table .local-warm { color: var(--warm); }
  .raw-table tr.stat-row-table td {
    font-weight: 700;
    border-top: 2px solid var(--border);
    background: rgba(255,255,255,0.02);
  }

  /* ---- Footer ---- */
  .footer {
    margin-top: 64px;
    padding-top: 20px;
    border-top: 1px solid var(--border);
    color: var(--text2);
    font-size: 12px;
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 8px;
  }
  .legend-inline { display: flex; gap: 16px; flex-wrap: wrap; align-items: center; }
  .legend-inline span { display: flex; align-items: center; gap: 5px; }
</style>
</head>
<body>

<header class="header">
  <div class="header-inner">
    <h1>CodeBurn <span class="accent">Performance Benchmark</span></h1>
    <div class="header-sub">npm v${installedVersion} (Phase 1 optimizations) vs local build (Phase 2 - SQLite session cache)</div>

    <div class="version-pills">
      <span class="pill pill-installed">
        <span class="pill-dot"></span>codeburn@${installedVersion} — npm install (global)
      </span>
      <span class="pill pill-local">
        <span class="pill-dot"></span>local ${localVersion} — dist/cli.js + better-sqlite3 cache
      </span>
    </div>

    <div class="headline-speedup">
      <span class="big">${globalWarmSpeedup}x</span>
      average warm-cache speedup across all commands
    </div>
  </div>
</header>

<div class="content">

  <div class="sysinfo">
    <div class="sysinfo-card">
      <div class="sysinfo-label">CPU</div>
      <div class="sysinfo-value" title="${cpuModel}">${cpuModel.replace('Apple ', '')}</div>
    </div>
    <div class="sysinfo-card">
      <div class="sysinfo-label">Cores</div>
      <div class="sysinfo-value">${cpuCount} logical</div>
    </div>
    <div class="sysinfo-card">
      <div class="sysinfo-label">RAM</div>
      <div class="sysinfo-value">${totalRamGB} GB</div>
    </div>
    <div class="sysinfo-card">
      <div class="sysinfo-label">Node.js</div>
      <div class="sysinfo-value">${nodeVersion}</div>
    </div>
    <div class="sysinfo-card">
      <div class="sysinfo-label">Runs / variant</div>
      <div class="sysinfo-value">${RUNS} (median)</div>
    </div>
    <div class="sysinfo-card">
      <div class="sysinfo-label">Platform</div>
      <div class="sysinfo-value">macOS (darwin)</div>
    </div>
    <div class="sysinfo-card">
      <div class="sysinfo-label">Measured</div>
      <div class="sysinfo-value">${timestamp.slice(0, 16).replace('T', ' ')} UTC</div>
    </div>
  </div>

  <h2 class="section-title">Summary</h2>
  <div class="summary-grid">
    ${summaryCards}
  </div>

  <h2 class="section-title">Per-command breakdown</h2>
  ${commandSections}

  <div class="footer">
    <span>Generated ${new Date(timestamp).toLocaleString()}</span>
    <div class="legend-inline">
      <span><span class="legend-dot installed"></span> v${installedVersion} npm (no SQLite cache)</span>
      <span><span class="legend-dot local-cold"></span> Phase 2 cold (cache miss)</span>
      <span><span class="legend-dot local-warm"></span> Phase 2 warm (cache hit)</span>
    </div>
  </div>

</div>
</body>
</html>`

writeFileSync(OUTPUT, html, 'utf8')
log(`\nWrote ${OUTPUT}`)
