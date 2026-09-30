# 0025: Validation goes through Standard Schema; TypeBox is the current implementation

**Status:** accepted, 2026-09-29; builds on ADR 0006, 0020 and 0021

## Context

The workbench validates in four places: server-function inputs (ADR 0020), form values (ADR 0021),
the sealed session read back from its store, and a few type guards over search parameters. Each
called TypeBox's `Value` API directly, fifteen files imported the library, and the friendly field
messages were tied to TypeBox's own error kinds. Moving to another validator (Zod, Valibot, a later
TypeBox) would have meant touching every one of those call sites as well as the schemas themselves.

Standard Schema is a small interface that the common TypeScript validators implement: a
`~standard.validate(input)` that returns either the output value or a list of issues, each a message
and a path. TanStack Form and TanStack Start already accept it. TypeBox 0.34 does not implement it.

## Decision

- One module, `src/validation/validate.ts`, is the only way the app validates. It accepts any
  Standard Schema and exposes `validate` (a value or field errors keyed by dotted field name),
  `is` (a type guard) and the `Input` and `Output` types that replace `Static`. `parse`,
  `schemaValidator` and `parsed` are built on it; nothing outside `src/validation/` imports
  `@sinclair/typebox/value`.
- TypeBox schemas are adapted rather than wrapped at call sites: the module passes anything without
  a `~standard` property through `fromTypeBox` (`src/validation/typebox.ts`), which keeps today's
  behaviour (unknown fields dropped, defaults applied, the `uuid` and `email` formats registered)
  and turns TypeBox's error kinds into the messages shown under a field. Adapters are cached per
  schema.
- Validation is synchronous. A schema whose `validate` returns a promise is refused, because form
  submit validators and server-function validators run synchronously here.
- The schemas stay TypeBox: the generated contract schemas (`src/api/schemas.gen.ts`) and the
  hand-written input schemas in server functions and forms. Changing library means a new emitter
  backend and rewriting those definitions; it no longer means touching the code that validates.

## Consequences

- Easier: a schema from another library works everywhere today without an adapter; replacing
  TypeBox is confined to the schema definitions, the emitter and one adapter file; the messages
  live next to the library whose error kinds they read.
- Harder: messages are per library, since Standard Schema issues carry a message and a path but
  not which rule failed, so a new adapter must rebuild the "Please select the reason" wording;
  the session store and the template snapshot test now clean before checking, which is more
  permissive than the strict check they used; route code that lists enum values still reads
  TypeBox's `anyOf`, which is schema introspection and outside this interface.
