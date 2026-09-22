import { cva } from 'class-variance-authority'

/** Text inputs, selects and textareas share one look; invalid turns the border red. */
export const inputVariants = cva(
  'mt-1 block w-full rounded-md border px-2 py-1 text-sm focus:border-neutral-500 focus:outline-none',
  {
    variants: {
      invalid: { true: 'border-red-400', false: 'border-neutral-300' },
      mono: { true: 'font-mono', false: '' },
    },
    defaultVariants: { invalid: false, mono: false },
  },
)

/** A11y attributes an invalid field carries, pointing at its FieldError. */
export const invalidProps = (id: string, message: string | undefined) =>
  message ? { 'aria-invalid': true as const, 'aria-describedby': `${id}-error` } : {}
