import { cva } from 'class-variance-authority'

export const inputVariants = cva(
  'mt-1 block w-full border bg-card px-1 py-[2px] text-[11px] shadow-[inset_1px_1px_2px_rgb(0_0_0/0.12)] focus:border-ring focus:outline-none',
  {
    variants: {
      invalid: { true: 'border-destructive', false: 'border-input' },
      mono: { true: 'font-mono', false: '' },
    },
    defaultVariants: { invalid: false, mono: false },
  },
)

export const FieldError = ({ id, message }: { id: string; message?: string | undefined }) =>
  message ? (
    <p id={id} className="mt-0.5 text-[11px] text-destructive">
      {message}
    </p>
  ) : null
