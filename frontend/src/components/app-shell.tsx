import type { ReactNode } from 'react'

type Props = { title: string; children: ReactNode }

export function AppShell({ title, children }: Props) {
  return (
    <div className="mx-auto max-w-5xl px-6 py-8">
      <header className="mb-8 flex items-baseline justify-between border-b border-neutral-200 pb-4">
        <span className="text-sm font-medium tracking-wide text-neutral-500 uppercase">Dispute Engine</span>
        <h1 className="text-2xl font-semibold">{title}</h1>
      </header>
      <main>{children}</main>
    </div>
  )
}
