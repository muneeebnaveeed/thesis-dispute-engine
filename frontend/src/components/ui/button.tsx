import { cva, type VariantProps } from 'class-variance-authority'
import type { ButtonHTMLAttributes } from 'react'

import { cn } from '#/lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center border text-[11px] font-normal transition-colors disabled:opacity-60',
  {
    variants: {
      variant: {
        primary:
          'border-input bg-[image:var(--raised-primary)] text-foreground shadow-[inset_0_1px_0_rgb(255_255_255/0.6)] hover:bg-accent hover:bg-none',
        secondary:
          'border-input bg-[image:var(--raised)] text-foreground shadow-[inset_0_1px_0_rgb(255_255_255/0.6)] hover:bg-secondary hover:bg-none',
        link: 'border-transparent text-primary underline hover:text-accent-foreground',
        action:
          'border-input bg-[image:var(--raised)] font-mono text-foreground shadow-[inset_0_1px_0_rgb(255_255_255/0.6)] hover:bg-secondary hover:bg-none',
      },
      size: {
        md: 'px-3 py-[3px]',
        sm: 'px-2 py-[2px]',
        xs: 'px-1.5 py-px',
        bare: 'border-0 p-0 shadow-none',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
)

type Props = ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof buttonVariants>

export const Button = ({ className, variant, size, type = 'button', ...rest }: Props) => (
  <button type={type} className={cn(buttonVariants({ variant, size }), className)} {...rest} />
)
