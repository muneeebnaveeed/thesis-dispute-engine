import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { vi } from 'vitest'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="/">{children}</a>,
  useRouteContext: () => ({
    viewer: {
      name: 'Eszter Varga',
      firstName: 'Eszter',
      lastName: 'Varga',
      tenantSlug: 'otp',
      tenantId: 't',
      email: null,
      roles: ['tenant-admin'],
    },
  }),
  useRouter: () => ({ navigate: vi.fn<() => Promise<void>>() }),
  useRouterState: () => '/otp',
}))
vi.mock('#/server/functions/session', () => ({ logout: vi.fn<() => Promise<{ url: string }>>() }))
vi.mock('#/server/functions/identity', () => ({
  getBranding: vi.fn<() => Promise<unknown>>(() =>
    Promise.resolve({ data: { name: 'OTP Bank', logoDataUrl: null }, response: {} }),
  ),
  getMyAvatar: vi.fn<() => Promise<unknown>>(() =>
    Promise.resolve({ data: { dataUrl: null }, response: {} }),
  ),
  putTenantLogo: vi.fn<() => Promise<unknown>>(),
  putMyAvatar: vi.fn<() => Promise<unknown>>(),
}))

import { AppShell } from '#/components/layout/app-shell'

const shell = (title = 'Disputes') =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <AppShell title={title}>
        <p>body</p>
      </AppShell>
    </QueryClientProvider>,
  )

test('the shell shows the title, the content and the tenant navigation', async () => {
  shell()
  expect(screen.getByRole('heading', { name: 'Disputes' })).toBeInTheDocument()
  expect(screen.getByText('body')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Disputes' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Search' })).toBeInTheDocument()
  // the person's name reads once, next to their picture; signing out is the icon beside it
  expect(screen.getByText('Eszter Varga')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Replace your picture' })).toBeInTheDocument()
  expect(await screen.findByText('OTP Bank')).toBeInTheDocument()
  expect(screen.queryByText('Dispute Engine')).not.toBeInTheDocument()
})

test('admin destinations come last and only a tenant admin sees them', () => {
  shell()
  expect(screen.queryByText('Admin panel')).not.toBeInTheDocument()
  const keys = screen.getByRole('link', { name: 'Tenant keys' })
  const disputes = screen.getByRole('link', { name: 'Disputes' })
  expect(disputes.compareDocumentPosition(keys) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(screen.getByRole('link', { name: 'Email templates' })).toBeInTheDocument()
  // the logo is the admin's way in to replacing it
  expect(screen.getByRole('button', { name: 'Replace the organisation logo' })).toBeInTheDocument()
})
