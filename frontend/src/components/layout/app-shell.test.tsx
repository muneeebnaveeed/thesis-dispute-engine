import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { vi } from 'vitest'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="/">{children}</a>,
  useRouteContext: () => ({
    viewer: { name: 'analyst', tenantSlug: 'otp', tenantId: 't', email: null, roles: ['tenant-admin'] },
  }),
  useRouter: () => ({ navigate: vi.fn<() => Promise<void>>() }),
  useRouterState: () => '/otp',
}))
vi.mock('#/server/functions/session', () => ({ logout: vi.fn<() => Promise<{ url: string }>>() }))

import { AppShell } from '#/components/layout/app-shell'

test('the shell shows the title, the content and the tenant navigation', () => {
  render(
    <AppShell title="Disputes">
      <p>body</p>
    </AppShell>,
  )
  expect(screen.getByRole('heading', { name: 'Disputes' })).toBeInTheDocument()
  expect(screen.getByText('body')).toBeInTheDocument()
  expect(screen.getByText('otp')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Disputes' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Search' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Sign out, analyst/ })).toBeInTheDocument()
})

test('the admin panel is the last group and only a tenant admin sees it', () => {
  render(<AppShell title="Disputes">body</AppShell>)
  const admin = screen.getByText('Admin panel')
  expect(admin).toBeInTheDocument()
  const keys = screen.getByRole('link', { name: 'Tenant keys' })
  const disputes = screen.getByRole('link', { name: 'Disputes' })
  expect(disputes.compareDocumentPosition(keys) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(screen.getByRole('link', { name: 'Email templates' })).toBeInTheDocument()
})
