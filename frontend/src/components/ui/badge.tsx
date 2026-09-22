import { cva, type VariantProps } from 'class-variance-authority'
import type { HTMLAttributes } from 'react'

import { cn } from '#/lib/utils'

const badgeVariants = cva('rounded px-1.5 py-0.5 text-xs font-medium', {
  variants: {
    tone: {
      neutral: 'bg-neutral-100 text-neutral-800',
      muted: 'bg-neutral-100 text-neutral-500',
      good: 'bg-emerald-100 text-emerald-900',
      warn: 'bg-amber-100 text-amber-900',
      bad: 'bg-red-100 text-red-900',
    },
  },
  defaultVariants: { tone: 'neutral' },
})

export type BadgeTone = NonNullable<VariantProps<typeof badgeVariants>['tone']>

type Props = HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>

export const Badge = ({ className, tone, ...rest }: Props) => (
  <span className={cn(badgeVariants({ tone }), className)} {...rest} />
)
