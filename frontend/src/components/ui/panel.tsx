import type { ReactNode } from 'react'

import { cn } from '#/lib/utils'

// The box everything of the period lived in: a white body under a grey heading bar, bordered and barely raised.
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
  <section
    className={cn('rounded-sm border border-border bg-card shadow-[0_1px_1px_rgb(0_0_0/0.05)]', className)}
  >
    <header className="flex flex-wrap items-center justify-between gap-2 rounded-t-sm border-b border-border bg-[image:var(--panel-heading)] px-4 py-2.5">
      <h2 className="text-[15px] font-medium text-foreground">{title}</h2>
      {actions}
    </header>
    <div className={cn('px-4 py-3', bodyClassName)}>{children}</div>
  </section>
)
