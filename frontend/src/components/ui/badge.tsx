import { cva, type VariantProps } from 'class-variance-authority'
import type { HTMLAttributes } from 'react'

import { cn } from '#/lib/utils'

// the period's label: a solid block of colour with white text, never a soft tint
const badgeVariants = cva(
  'inline-block px-1 py-px text-[10px] font-bold tracking-wide text-[color:var(--tone-foreground)] uppercase',
  {
    variants: {
      tone: {
        neutral: 'bg-[color:var(--tone-neutral)]',
        muted: 'bg-[color:var(--tone-muted)]',
        good: 'bg-[color:var(--tone-good)]',
        warn: 'bg-[color:var(--tone-warn)]',
        bad: 'bg-[color:var(--tone-bad)]',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
)

export type BadgeTone = NonNullable<VariantProps<typeof badgeVariants>['tone']>

type Props = HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>

export const Badge = ({ className, tone, ...rest }: Props) => (
  <span className={cn(badgeVariants({ tone }), className)} {...rest} />
)
