import { cva, type VariantProps } from 'class-variance-authority'
import type { ButtonHTMLAttributes } from 'react'

import { cn } from '#/lib/utils'

export const buttonVariants = cva(
  'inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors disabled:opacity-50',
  {
    variants: {
      variant: {
        primary: 'bg-neutral-900 text-white hover:bg-neutral-700',
        secondary: 'border border-neutral-300 bg-white hover:bg-neutral-100',
        link: 'underline hover:text-neutral-900',
        action: 'border border-neutral-300 bg-white font-mono hover:bg-neutral-100',
      },
      size: {
        md: 'px-4 py-2',
        sm: 'px-3 py-1.5',
        xs: 'px-2 py-1 text-xs',
        bare: 'p-0',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
)

type Props = ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof buttonVariants>

export const Button = ({ className, variant, size, type = 'button', ...rest }: Props) => (
  <button type={type} className={cn(buttonVariants({ variant, size }), className)} {...rest} />
)
