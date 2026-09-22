import { cva } from 'class-variance-authority'

export const inputVariants = cva(
  'mt-1 block w-full rounded-sm border bg-card px-2 py-1 text-[13px] shadow-[inset_0_1px_1px_rgb(0_0_0/0.075)] focus:border-ring focus:shadow-[inset_0_1px_1px_rgb(0_0_0/0.075),0_0_8px_rgb(102_175_233/0.6)] focus:outline-none',
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
    <p id={id} className="mt-1 text-xs text-destructive">
      {message}
    </p>
  ) : null
