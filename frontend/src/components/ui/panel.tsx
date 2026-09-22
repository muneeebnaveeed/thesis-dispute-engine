import type { ReactNode } from 'react'

import { cn } from '#/lib/utils'

// The box everything of the period lived in: a white body under a gradient heading, ruled in the chrome's own blue.
export const Panel = ({
  title,
  actions,
  children,
  className,
  bodyClassName,
}: {
  title: string
  actions?: ReactNode
  children: ReactNode
  className?: string
  bodyClassName?: string
}) => (
  <section className={cn('border border-border bg-card', className)}>
    <header className="flex flex-wrap items-center justify-between gap-2 border-b border-border bg-[image:var(--panel-heading)] px-2 py-1">
      <h2 className="text-[11px] font-bold text-primary">{title}</h2>
      {actions}
    </header>
    <div className={cn('p-2', bodyClassName)}>{children}</div>
  </section>
)
