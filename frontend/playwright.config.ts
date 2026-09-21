import { defineConfig, devices } from '@playwright/test'

// Runs against the real stack: Postgres, API, Keycloak (make auth-up) and this app's production build.
// E2E_BASE_URL points at an already running frontend; otherwise the config builds and starts one.
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:3002'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  ...(process.env.E2E_BASE_URL
    ? {}
    : {
        webServer: {
          command: 'pnpm build && pnpm start',
          url: baseURL,
          reuseExistingServer: !process.env.CI,
          timeout: 240_000,
          env: { PORT: '3002' },
        },
      }),
})
