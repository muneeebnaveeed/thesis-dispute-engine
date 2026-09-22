# 0021: Forms are TanStack Form, validated by the contract's schemas, with server refusals landing on the field

**Status:** accepted, 2026-09-22; builds on ADR 0012 and 0020

## Context

Every form in the workbench was hand-rolled: a `FormData` read, a TypeBox check, local state for
the values that needed a live preview, and a merge of client and server field errors that each page
did slightly differently. Seven forms had four ways of showing an error. The choice was between
React Hook Form and TanStack Form. Hook Form is built around uncontrolled inputs registered by ref,
which fits poorly with the controlled, derived inputs here (a multiselect held as a comma list, a
preview rendered from the draft on every keystroke) and knows nothing of Start. TanStack Form is
headless and controlled, takes plain functions as validators, and sits in the stack the app already
runs on.

## Decision

- All forms use `useAppForm` from `src/forms/app-form.tsx`, a `createFormHook` instance with the
  app's bound field components (`TextField`, `TextareaField`, `SelectField`, `CheckboxField`,
  `CheckboxGroupField`) and `SubmitButton`. A field owns its label, its cva input class, its
  `aria-invalid`/`aria-describedby` wiring and its `FieldError` (id `<name>-error`, dots replaced by
  dashes), so a page never assembles those by hand.
- Structural validation is the contract's TypeBox schema through `schemaValidator(schema)` as the
  form's `onSubmit` validator; `parsed(schema, value)` yields the cleaned, defaulted value the
  mutation sends. TypeBox 0.34 does not implement Standard Schema, so the adapter is a
  five-line function rather than a package.
- A form that writes calls its mutation inside `submitTo(formApi, () => mutation.mutateAsync(...))`.
  A validation refusal from the API (ADR 0012 kind `validation`) is placed on the fields it names
  through `setErrorMap`, so a server-side field error looks exactly like a client-side one; every
  other failure stays in the mutation's `failure` for the banner. Field names therefore follow
  the API's error pointers (`answers.<id>`, `fields.<id>`), never a UI-local name.
- Live previews read the draft with `useStore(form.store, s => s.values)`; a button that decides
  which facts travel (the dispute actions) passes the event as submit meta.

## Consequences

- Easier: one shape for every form; server and client errors indistinguishable to the person; the
  preview and the submitted value come from the same store; a new form is default values, a schema
  and a mutation.
- Harder: TanStack Form's types are heavy and `createFormHook` fixes the field vocabulary, so a
  one-off input becomes a bound component rather than a bare `<input>`; the submit-meta pattern for
  multi-button forms is less obvious than one handler per button.
