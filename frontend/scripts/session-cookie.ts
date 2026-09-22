// Prints a signed-in de_session cookie for the seeded analyst, for load runs that read the workbench the way a
// browser does (backend/cmd/eval --workbench --session). Usage: node scripts/session-cookie.ts [slug] [baseURL]
import { chromium } from '@playwright/test'

const slug = process.argv[2] ?? 'otp'
const baseURL = process.argv[3] ?? 'http://localhost:3002'
const browser = await chromium.launch()
const page = await browser.newPage()
await page.goto(`${baseURL}/${slug}`)
await page.waitForURL(/\/protocol\/openid-connect\/|localhost:3002/)
if (page.url().includes('/protocol/openid-connect/')) {
  await page.fill('#username', 'analyst')
  await page.fill('#password', 'analyst')
  await page.click('#kc-login')
}
await page.waitForURL((url) => url.host === new URL(baseURL).host && !url.pathname.startsWith('/auth/'))
const session = (await page.context().cookies()).find((cookie) => cookie.name === 'de_session')
await browser.close()
if (!session) {
  console.error('no de_session cookie after sign-in')
  process.exit(1)
}
console.log(session.value)
