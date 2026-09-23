// Captures the workbench figures the thesis register asks for, reproducibly, into docs/thesis/latex/figures.
// Needs the stack up (make auth:up and the workbench on :3002). Usage: pnpm screenshot [name...]
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { chromium, type Page } from '@playwright/test'

const BASE = process.env.APP_URL ?? 'http://localhost:3002'
const TENANT = process.env.SCREENSHOT_TENANT ?? 'otp'
const OUT = join(dirname(fileURLToPath(import.meta.url)), '../../docs/thesis/latex/figures')
// JPEG rather than the smaller WebP because pdflatex cannot include WebP and the thesis is the main reader;
// at this quality the small text stays sharp and a figure is about a tenth of the PNG
const QUALITY = 72

type Figure = { path: string; height?: number; element?: string; prepare?: (page: Page) => Promise<void> }

const figures: Record<string, Figure> = {
  workbench: { path: `/${TENANT}`, height: 900 },
  search: { path: `/${TENANT}/search`, height: 700 },
  sidebar: { path: `/${TENANT}`, height: 720, element: '[data-slot=sidebar-container]' },
  dispute: {
    path: `/${TENANT}`,
    height: 1100,
    prepare: async (page) => {
      await page.getByRole('table', { name: 'Disputes' }).getByRole('link').first().click()
      // the router changes the URL before it paints, so wait for the page itself, not the address
      await page.getByRole('heading', { name: /^Dispute [0-9a-f]{8}$/ }).waitFor()
    },
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
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })

await page.goto(`${BASE}/${TENANT}`)
await page.waitForURL(/\/protocol\/openid-connect\/|localhost/)
if (page.url().includes('/protocol/openid-connect/')) {
  await page.fill('#username', process.env.SCREENSHOT_USER ?? 'analyst')
  await page.fill('#password', process.env.SCREENSHOT_PASSWORD ?? 'analyst')
  await page.click('#kc-login')
}
await page.waitForURL((url) => url.host === new URL(BASE).host && !url.pathname.startsWith('/auth/'))

for (const [name, figure] of chosen) {
  await page.setViewportSize({ width: 1280, height: figure.height ?? 900 })
  await page.goto(BASE + figure.path, { waitUntil: 'networkidle' })
  await figure.prepare?.(page)
  await page.waitForLoadState('networkidle')
  const file = join(OUT, `${name}.jpg`)
  const shot = figure.element ? page.locator(figure.element).first() : page
  await shot.screenshot({ path: file, type: 'jpeg', quality: QUALITY })
  console.log(`docs/thesis/latex/figures/${name}.jpg`)
}

await browser.close()
