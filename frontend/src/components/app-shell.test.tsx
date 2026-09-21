import { render, screen } from '@testing-library/react'

import { AppShell } from './app-shell'

test('renders the page title and its content', () => {
  render(
    <AppShell title="Disputes">
      <p>body</p>
    </AppShell>,
  )
  expect(screen.getByRole('heading', { name: 'Disputes' })).toBeInTheDocument()
  expect(screen.getByText('body')).toBeInTheDocument()
})
