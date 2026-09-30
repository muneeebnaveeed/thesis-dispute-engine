import { render } from '@testing-library/react'
import type { ReactNode } from 'react'
import { expect, test, vi } from 'vitest'

import { RouteError } from './route-error'

const check = vi.fn<() => Promise<boolean>>(async () => false)
vi.mock('#/lib/stale-build', () => ({ staleBuildGuard: () => ({ check }) }))
vi.mock('#/components/layout/app-shell', () => ({
  AppShell: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}))

test('a page that failed to load asks whether the build went stale', () => {
  render(<RouteError error={new Error('HTTPError')} reset={() => undefined} info={{ componentStack: '' }} />)
  expect(check).toHaveBeenCalledTimes(1)
})
