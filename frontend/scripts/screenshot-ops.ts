// Captures the operational figures (dashboard, one trace, a CI run) into docs/thesis/latex/figures.
// Needs make demo (or otel:up) with some traffic, e.g. make thesis:eval. Usage: pnpm screenshot:ops [name...]
// TRACE_ID picks the trace; CI_RUN_URL the GitHub Actions run (public pages, no sign-in).
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { chromium, type Page } from '@playwright/test'

const GRAFANA = process.env.GRAFANA_URL ?? 'http://localhost:3001'
const OUT = join(dirname(fileURLToPath(import.meta.url)), '../../docs/thesis/latex/figures')
const QUALITY = 72

// crop picks the element that is the figure: the smallest one whose text has every include and no exclude
type Crop = { include: string[]; exclude?: string[] }
type Figure = {
  url: () => string
  height: number
  signIn?: boolean
  crop?: Crop
  settle?: (page: Page) => Promise<void>
}

const explore = (traceId: string) =>
  `${GRAFANA}/explore?theme=light&schemaVersion=1&panes=${encodeURIComponent(
    JSON.stringify({
      a: {
        datasource: 'tempo',
        queries: [
          { refId: 'A', datasource: { type: 'tempo', uid: 'tempo' }, queryType: 'traceql', query: traceId },
        ],
        range: { from: 'now-6h', to: 'now' },
      },
    }),
  )}`

const figures: Record<string, Figure> = {
  dashboard: {
    url: () => `${GRAFANA}/d/dispute-engine/dispute-engine?orgId=1&from=now-30m&to=now&kiosk&theme=light`,
    height: 1000,
    signIn: true,
    settle: (page) => page.waitForTimeout(3000),
  },
  trace: {
    url: () => {
      if (!process.env.TRACE_ID) throw new Error('set TRACE_ID')
      return explore(process.env.TRACE_ID)
    },
    height: 1500,
    signIn: true,
    crop: { include: ['Trace ID', 'Service & Operation'], exclude: ['Query history'] },
    settle: (page) => page.waitForTimeout(3000),
  },
  ci: {
    url: () => {
      if (!process.env.CI_RUN_URL) throw new Error('set CI_RUN_URL')
      return process.env.CI_RUN_URL
    },
    height: 900,
    crop: { include: ['ci.yml', 'frontend image'] },
    settle: (page) => page.waitForTimeout(2000),
  },
}

const wanted = process.argv.slice(2)
const chosen = Object.entries(figures).filter(([name]) => wanted.length === 0 || wanted.includes(name))
if (chosen.length === 0) {
  console.error(`no such figure; known: ${Object.keys(figures).join(', ')}`)
  process.exit(1)
}

mkdirSync(OUT, { recursive: true })
const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1400, height: 900 } })
let signedIn = false

for (const [name, figure] of chosen) {
  if (figure.signIn && !signedIn) {
    // the local Grafana's development login, from deploy/compose.yml
    await page.request.post(`${GRAFANA}/login`, {
      data: { user: process.env.GRAFANA_USER ?? 'admin', password: process.env.GRAFANA_PASSWORD ?? 'admin' },
    })
    signedIn = true
  }
  await page.setViewportSize({ width: 1400, height: figure.height })
  await page.goto(figure.url(), { waitUntil: 'networkidle' })
  await figure.settle?.(page)
  const file = join(OUT, `${name}.jpg`)
  if (figure.crop) {
    const crop = figure.crop
    const box = await page.evaluate(({ include, exclude = [] }) => {
      let best: DOMRect | undefined
      for (const el of document.querySelectorAll('div, section')) {
        const text = (el as HTMLElement).innerText ?? ''
        if (!include.every((n) => text.includes(n)) || exclude.some((n) => text.includes(n))) continue
        const r = el.getBoundingClientRect()
        if (r.width > 0 && (!best || r.width * r.height < best.width * best.height)) best = r
      }
      return best && { x: best.x, y: best.y, width: best.width, height: best.height }
    }, crop)
    if (!box) throw new Error(`${name}: nothing contains ${crop.include.join(', ')}`)
    await page.screenshot({ path: file, type: 'jpeg', quality: QUALITY, clip: box })
  } else {
    await page.screenshot({ path: file, type: 'jpeg', quality: QUALITY })
  }
  console.log(`docs/thesis/latex/figures/${name}.jpg`)
}

await browser.close()
