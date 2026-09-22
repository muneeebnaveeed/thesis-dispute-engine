import { cva, type VariantProps } from 'class-variance-authority'
import type { HTMLAttributes } from 'react'

import { cn } from '#/lib/utils'

// the period's label: a solid block of colour with white text, never a soft tint
const badgeVariants = cva(
  'inline-block rounded-sm px-1.5 py-0.5 text-[11px] font-bold tracking-wide text-white uppercase',
  {
    variants: {
      tone: {
        neutral: 'bg-[#777777]',
        muted: 'bg-[#b4b4b4]',
        good: 'bg-[#5cb85c]',
        warn: 'bg-[#f0ad4e]',
        bad: 'bg-[#d9534f]',
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
