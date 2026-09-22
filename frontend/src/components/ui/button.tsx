import { cva, type VariantProps } from 'class-variance-authority'
import type { ButtonHTMLAttributes } from 'react'

import { cn } from '#/lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center rounded-sm border text-[13px] font-normal transition-colors disabled:opacity-65',
  {
    variants: {
      variant: {
        primary:
          'border-[#2b669a] bg-primary bg-[image:var(--raised-primary)] text-primary-foreground shadow-[inset_0_1px_0_rgb(255_255_255/0.15),0_1px_1px_rgb(0_0_0/0.075)] [text-shadow:0_-1px_0_rgb(0_0_0/0.2)] hover:bg-[#286090] hover:bg-none',
        secondary:
          'border-input bg-card bg-[image:var(--raised)] text-foreground shadow-[0_1px_1px_rgb(0_0_0/0.075)] hover:bg-secondary hover:bg-none',
        link: 'border-transparent text-primary underline hover:text-[#23527c]',
        action:
          'border-input bg-card bg-[image:var(--raised)] font-mono text-foreground shadow-[0_1px_1px_rgb(0_0_0/0.075)] hover:bg-secondary hover:bg-none',
      },
      size: {
        md: 'px-3 py-1.5',
        sm: 'px-2.5 py-1',
        xs: 'px-2 py-0.5 text-xs',
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
