import { createFormHook, createFormHookContexts, type AnyFormApi } from '@tanstack/react-form'
import type { ComponentProps, FormEvent, InputHTMLAttributes, ReactNode } from 'react'

import { Button } from '#/components/ui/button'
import { FieldError, inputVariants } from '#/components/ui/field'
import { cn } from '#/lib/cn'
import { asFieldErrors } from '#/queries/use-server-mutation'

const { fieldContext, formContext, useFieldContext, useFormContext } = createFormHookContexts()

const errorId = (fieldName: string) => `${fieldName.replaceAll('.', '-')}-error`

const firstError = (errors: unknown[]): string | undefined => {
  const first = errors.find((error) => typeof error === 'string' && error !== '')
  return typeof first === 'string' ? first : undefined
}

// what every bound input shares: the value, the change handler, and the a11y wiring of its first error
const useBoundInput = () => {
  const field = useFieldContext<string>()
  const message = firstError(field.state.meta.errors)
  return {
    field,
    message,
    id: errorId(field.name),
    inputProps: {
      name: field.name,
      value: field.state.value,
      onBlur: field.handleBlur,
      ...(message ? { 'aria-invalid': true as const, 'aria-describedby': errorId(field.name) } : {}),
    },
  }
}

type LabelProps = { label: ReactNode; hint?: ReactNode; className?: string }

const TextField = ({
  label,
  hint,
  className,
  mono = false,
  inputClassName,
  ...input
}: LabelProps & { mono?: boolean; inputClassName?: string } & Omit<
    InputHTMLAttributes<HTMLInputElement>,
    'value' | 'onChange' | 'onBlur' | 'name' | 'className'
  >) => {
  const { field, message, id, inputProps } = useBoundInput()
  return (
    <label className={cn('block text-sm', className)}>
      {label}
      {hint && <span className="text-neutral-400"> {hint}</span>}
      <input
        {...input}
        {...inputProps}
        onChange={(event) => field.handleChange(event.target.value)}
        className={cn(inputVariants({ invalid: Boolean(message), mono }), inputClassName)}
      />
      <FieldError id={id} message={message} />
    </label>
  )
}

const TextareaField = ({
  label,
  hint,
  className,
  rows = 3,
  mono = false,
  inputClassName,
  disabled,
}: LabelProps & { rows?: number; mono?: boolean; inputClassName?: string; disabled?: boolean }) => {
  const { field, message, id, inputProps } = useBoundInput()
  return (
    <label className={cn('block text-sm', className)}>
      {label}
      {hint && <span className="text-neutral-400"> {hint}</span>}
      <textarea
        {...inputProps}
        rows={rows}
        disabled={disabled}
        onChange={(event) => field.handleChange(event.target.value)}
        className={cn(inputVariants({ invalid: Boolean(message), mono }), inputClassName)}
      />
      <FieldError id={id} message={message} />
    </label>
  )
}

type Option = { value: string; label: string }

const SelectField = ({
  label,
  hint,
  className,
  options,
  placeholder,
  mono = false,
  inputClassName,
  disabled,
}: LabelProps & {
  options: Option[]
  placeholder?: string
  mono?: boolean
  inputClassName?: string
  disabled?: boolean
}) => {
  const { field, message, id, inputProps } = useBoundInput()
  return (
    <label className={cn('block text-sm', className)}>
      {label}
      {hint && <span className="text-neutral-400"> {hint}</span>}
      <select
        {...inputProps}
        disabled={disabled}
        onChange={(event) => field.handleChange(event.target.value)}
        className={cn(inputVariants({ invalid: Boolean(message), mono }), inputClassName)}
      >
        {placeholder !== undefined && <option value="">{placeholder}</option>}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <FieldError id={id} message={message} />
    </label>
  )
}

const CheckboxField = ({ label, className }: { label: ReactNode; className?: string }) => {
  const field = useFieldContext<boolean>()
  return (
    <label className={cn('flex items-center gap-1 text-sm', className)}>
      <input
        type="checkbox"
        name={field.name}
        checked={field.state.value}
        onBlur={field.handleBlur}
        onChange={(event) => field.handleChange(event.target.checked)}
      />
      {label}
    </label>
  )
}

// one string field holding a comma-separated set of keys, edited as checkboxes
const CheckboxGroupField = ({
  label,
  options,
}: {
  label: ReactNode
  options: { key: string; label: string; description?: string }[]
}) => {
  const { field, message, id } = useBoundInput()
  const chosen = new Set(field.state.value.split(',').filter(Boolean))
  const toggle = (key: string, on: boolean) => {
    const next = new Set(chosen)
    if (on) next.add(key)
    else next.delete(key)
    field.handleChange([...next].join(','))
  }
  return (
    <fieldset className="text-sm" aria-describedby={message ? id : undefined}>
      <legend className="mb-1">{label}</legend>
      <div className="space-y-1">
        {options.map((option) => (
          <label key={option.key} className="flex items-start gap-2">
            <input
              type="checkbox"
              className="mt-1"
              checked={chosen.has(option.key)}
              onChange={(event) => toggle(option.key, event.target.checked)}
            />
            <span>
              {option.label}
              {option.description && (
                <span className="block text-xs text-neutral-500">{option.description}</span>
              )}
            </span>
          </label>
        ))}
      </div>
      <FieldError id={id} message={message} />
    </fieldset>
  )
}

const SubmitButton = ({
  children,
  busy = false,
  ...button
}: { children: ReactNode; busy?: boolean } & Omit<ComponentProps<typeof Button>, 'type' | 'children'>) => {
  const form = useFormContext()
  return (
    <form.Subscribe selector={(state) => state.isSubmitting}>
      {(isSubmitting) => (
        <Button type="submit" disabled={busy || isSubmitting} {...button}>
          {children}
        </Button>
      )}
    </form.Subscribe>
  )
}

const fieldComponents = { TextField, TextareaField, SelectField, CheckboxField, CheckboxGroupField }

// what a render prop receives as `field`, for components that pick the input by data
export type BoundFieldComponents = typeof fieldComponents

export const { useAppForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents,
  formComponents: { SubmitButton },
})

// the submit handler every <form> uses: TanStack Form owns validation, the browser never posts
export const submitting =
  (form: { handleSubmit: () => Promise<void> }) =>
  (event: FormEvent<HTMLFormElement>): void => {
    event.preventDefault()
    event.stopPropagation()
    void form.handleSubmit()
  }

// runs a mutation from a form's onSubmit: a validation refusal lands on the fields it names, every other failure
// stays in the mutation's `failure` for the banner
export const submitTo = async (form: AnyFormApi, run: () => Promise<unknown>): Promise<void> => {
  try {
    await run()
    form.setErrorMap({ onSubmit: undefined })
  } catch (thrown) {
    const fieldErrors = asFieldErrors(thrown)
    if (fieldErrors) form.setErrorMap({ onSubmit: fieldErrors })
  }
}
