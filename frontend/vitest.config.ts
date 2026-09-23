// Separate from vite.config.ts on purpose: the Start and Nitro plugins spin up servers the unit tests do not need.
import { defineConfig } from 'vitest/config'
import viteReact from '@vitejs/plugin-react'

export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [viteReact()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      // Only the layers that hold logic. Routes and components are composition, covered by e2e;
      // src/api and routeTree.gen.ts are generated.
      include: ['src/server/**', 'src/lib/**'],
      reporter: ['text', 'json-summary'],
      reportsDirectory: './coverage',
    },
  },
})
