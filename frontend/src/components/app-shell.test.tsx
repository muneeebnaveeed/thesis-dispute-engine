import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { vi } from 'vitest'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="/">{children}</a>,
  useRouteContext: () => ({
    viewer: { name: 'analyst', tenantSlug: 'otp', tenantId: 't', email: null, roles: [] },
  }),
  useRouter: () => ({ navigate: vi.fn<() => Promise<void>>() }),
}))
vi.mock('#/server/auth/session', () => ({ logout: vi.fn<() => Promise<{ url: string }>>() }))
vi.mock('#/api/browser', () => ({ forgetToken: vi.fn<() => void>() }))

import { AppShell } from './app-shell'

test('renders the title, the content and who is signed in', () => {
  render(
    <AppShell title="Disputes">
      <p>body</p>
    </AppShell>,
  )
  expect(screen.getByRole('heading', { name: 'Disputes' })).toBeInTheDocument()
  expect(screen.getByText('body')).toBeInTheDocument()
  expect(screen.getByText(/analyst/)).toBeInTheDocument()
  expect(screen.getByText(/otp/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
})
