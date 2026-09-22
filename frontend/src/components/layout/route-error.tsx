import type { ErrorComponentProps } from '@tanstack/react-router'

import { AppShell } from '#/components/layout/app-shell'

/** The last line: an exception nothing else caught. Says so plainly, offers a reload, never shows a stack. */
export const RouteError = ({ error, reset }: ErrorComponentProps) => {
  const message = error instanceof Error ? error.message : 'unexpected error'
  return (
    <AppShell title="Something went wrong">
      <div role="alert" className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-900">
        <p className="font-medium">This page could not be shown.</p>
        <p className="mt-1 font-mono text-xs opacity-80">{message}</p>
        <button type="button" onClick={reset} className="mt-3 underline">
          Try again
        </button>
      </div>
    </AppShell>
  )
}
