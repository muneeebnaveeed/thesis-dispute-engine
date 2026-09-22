import { Dialog as DialogPrimitive } from 'radix-ui'
import type { ReactNode } from 'react'

import { cn } from '#/lib/utils'

// The modal of the period: a title bar with its tool buttons, a white body, and the commands on a footer rail.
export const Window = ({
  open,
  onOpenChange,
  title,
  description,
  footer,
  children,
  className,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  footer?: ReactNode
  children: ReactNode
  className?: string
}) => (
  <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-40 bg-white/50" />
      <DialogPrimitive.Content
        className={cn(
          'fixed top-28 left-1/2 z-50 w-[28rem] -translate-x-1/2 border border-border bg-background shadow-[0_4px_12px_rgb(0_0_0/0.35)]',
          className,
        )}
      >
        <div className="flex items-center justify-between border-b border-border bg-[image:var(--panel-heading)] px-1.5 py-1">
          <DialogPrimitive.Title className="text-[11px] font-bold text-primary">
            {title}
          </DialogPrimitive.Title>
          <DialogPrimitive.Close
            aria-label="Close"
            className="h-[13px] w-[13px] border border-border bg-card text-[9px] leading-[11px] text-primary"
          >
            x
          </DialogPrimitive.Close>
        </div>
        <DialogPrimitive.Description className="sr-only">{description ?? title}</DialogPrimitive.Description>
        <div className="bg-card p-2.5">{children}</div>
        {footer && (
          <div className="flex justify-end gap-1.5 border-t border-border bg-[image:var(--toolbar)] px-1.5 py-1.5">
            {footer}
          </div>
        )}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  </DialogPrimitive.Root>
)
