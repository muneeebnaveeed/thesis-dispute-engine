// Generated from docs/api/openapi.yaml by scripts/generate-api.ts. Do not edit.

import { Type, type Static } from '@sinclair/typebox'
import type { components } from './schema.gen'

// Each schema is checked both ways against the openapi-typescript type, so the two generated files cannot drift.
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : never) : never

export const DisputeEvent = Type.Union([
  Type.Literal('OPEN_INVESTIGATION'),
  Type.Literal('SEND_QUESTIONNAIRE'),
  Type.Literal('RECEIVE_QUESTIONNAIRE'),
  Type.Literal('ISSUE_REFUND'),
  Type.Literal('FILE_CHARGEBACK'),
  Type.Literal('ACKNOWLEDGE_CHARGEBACK'),
  Type.Literal('SUBMIT_EVIDENCE'),
  Type.Literal('WIN_CHARGEBACK'),
  Type.Literal('LOSE_CHARGEBACK'),
  Type.Literal('ISSUE_FINAL_CREDIT'),
  Type.Literal('REVERSE_PROVISIONAL_CREDIT'),
  Type.Literal('CLOSE'),
  Type.Literal('APPEAL'),
])
export type DisputeEvent = Static<typeof DisputeEvent>
const _DisputeEvent: Same<DisputeEvent, components['schemas']['DisputeEvent']> = true
void _DisputeEvent

export const ApplyEventRequest = Type.Object({
  event: DisputeEvent,
  actor: Type.Optional(Type.String({ default: 'system' })),
  payload: Type.Optional(
    Type.Record(Type.String(), Type.Unknown(), {
      description: 'Event-specific facts, stored verbatim on the log entry.',
    }),
  ),
})
export type ApplyEventRequest = Static<typeof ApplyEventRequest>
const _ApplyEventRequest: Same<ApplyEventRequest, components['schemas']['ApplyEventRequest']> = true
void _ApplyEventRequest

export const CreateDisputeRequest = Type.Object({
  transactionId: Type.String({ format: 'uuid' }),
  actor: Type.Optional(
    Type.String({ description: 'Who opened it; defaults to customer.', default: 'customer' }),
  ),
})
export type CreateDisputeRequest = Static<typeof CreateDisputeRequest>
const _CreateDisputeRequest: Same<CreateDisputeRequest, components['schemas']['CreateDisputeRequest']> = true
void _CreateDisputeRequest

export const CreateTenantKeyRequest = Type.Object({
  label: Type.String({ description: 'Which system will hold it.', minLength: 1, maxLength: 80 }),
  expiresAt: Type.Optional(
    Type.String({ description: 'Optional; the key stops working after this.', format: 'date-time' }),
  ),
})
export type CreateTenantKeyRequest = Static<typeof CreateTenantKeyRequest>
const _CreateTenantKeyRequest: Same<CreateTenantKeyRequest, components['schemas']['CreateTenantKeyRequest']> =
  true
void _CreateTenantKeyRequest

export const Regime = Type.Union([
  Type.Literal('EU_SEPA_DIRECT_DEBIT'),
  Type.Literal('EU_PSD2_CARD'),
  Type.Literal('US_REG_E'),
  Type.Literal('US_REG_Z'),
])
export type Regime = Static<typeof Regime>
const _Regime: Same<Regime, components['schemas']['Regime']> = true
void _Regime

export const DisputeState = Type.Union([
  Type.Literal('INITIATED'),
  Type.Literal('INVESTIGATING'),
  Type.Literal('QUESTIONNAIRE_SENT'),
  Type.Literal('QUESTIONNAIRE_RECEIVED'),
  Type.Literal('PROVISIONAL_CREDIT_ISSUED'),
  Type.Literal('FAST_REFUND_ISSUED'),
  Type.Literal('SEPA_NQA_REFUND_ISSUED'),
  Type.Literal('CHARGEBACK_FILED'),
  Type.Literal('CHARGEBACK_ACKNOWLEDGED'),
  Type.Literal('EVIDENCE_SUBMITTED'),
  Type.Literal('CHARGEBACK_WON'),
  Type.Literal('CHARGEBACK_LOST'),
  Type.Literal('FINAL_CREDIT_ISSUED'),
  Type.Literal('PROVISIONAL_CREDIT_REVERSED'),
  Type.Literal('CLOSED'),
])
export type DisputeState = Static<typeof DisputeState>
const _DisputeState: Same<DisputeState, components['schemas']['DisputeState']> = true
void _DisputeState

export const LoggedEvent = Type.Object({
  seq: Type.Integer(),
  event: Type.String({ description: 'A DisputeEvent, or OPENED for entry 1.' }),
  fromState: Type.String(),
  toState: DisputeState,
  actor: Type.String(),
  payload: Type.Record(Type.String(), Type.Unknown()),
  traceId: Type.Optional(Type.String()),
  occurredAt: Type.String({ format: 'date-time' }),
})
export type LoggedEvent = Static<typeof LoggedEvent>
const _LoggedEvent: Same<LoggedEvent, components['schemas']['LoggedEvent']> = true
void _LoggedEvent

export const Dispute = Type.Object({
  id: Type.String({ format: 'uuid' }),
  regime: Regime,
  state: DisputeState,
  appeals: Type.Integer(),
  version: Type.Integer({ format: 'int64' }),
  transactionId: Type.String({ format: 'uuid' }),
  accountId: Type.String({ format: 'uuid' }),
  disputedAmount: Type.String({ description: 'Decimal as a string; never a float.', example: '125.4000' }),
  currency: Type.String({ minLength: 3, maxLength: 3 }),
  openedAt: Type.String({ format: 'date-time' }),
  updatedAt: Type.String({ format: 'date-time' }),
  allowedEvents: Type.Array(DisputeEvent),
  events: Type.Array(LoggedEvent),
})
export type Dispute = Static<typeof Dispute>
const _Dispute: Same<Dispute, components['schemas']['Dispute']> = true
void _Dispute

export const DisputeSummary = Type.Object({
  id: Type.String({ format: 'uuid' }),
  regime: Regime,
  state: DisputeState,
  transactionId: Type.String({ format: 'uuid' }),
  disputedAmount: Type.String({ description: 'Decimal as a string; never a float.' }),
  currency: Type.String({ minLength: 3, maxLength: 3 }),
  openedAt: Type.String({ format: 'date-time' }),
  updatedAt: Type.String({ format: 'date-time' }),
})
export type DisputeSummary = Static<typeof DisputeSummary>
const _DisputeSummary: Same<DisputeSummary, components['schemas']['DisputeSummary']> = true
void _DisputeSummary

export const DisputePage = Type.Object({
  items: Type.Array(DisputeSummary),
  nextCursor: Type.Optional(Type.String({ description: 'Present when there is another page.' })),
})
export type DisputePage = Static<typeof DisputePage>
const _DisputePage: Same<DisputePage, components['schemas']['DisputePage']> = true
void _DisputePage

export const ErrorCode = Type.Union(
  [
    Type.Literal('unauthenticated'),
    Type.Literal('forbidden'),
    Type.Literal('rate-limited'),
    Type.Literal('malformed-request'),
    Type.Literal('contract-violation'),
    Type.Literal('not-found'),
    Type.Literal('invalid-transition'),
    Type.Literal('appeals-exhausted'),
    Type.Literal('concurrent-update'),
    Type.Literal('idempotency-key-reuse'),
    Type.Literal('no-regime'),
    Type.Literal('unknown-regime'),
    Type.Literal('unavailable'),
    Type.Literal('internal'),
  ],
  { description: 'Stable machine-readable failure identifier; branch on this, never on text.' },
)
export type ErrorCode = Static<typeof ErrorCode>
const _ErrorCode: Same<ErrorCode, components['schemas']['ErrorCode']> = true
void _ErrorCode

export const FieldError = Type.Object({
  field: Type.String({ example: '/transactionId' }),
  message: Type.String(),
})
export type FieldError = Static<typeof FieldError>
const _FieldError: Same<FieldError, components['schemas']['FieldError']> = true
void _FieldError

export const Health = Type.Object({
  status: Type.Union([Type.Literal('ok'), Type.Literal('unavailable')]),
  checks: Type.Optional(Type.Record(Type.String(), Type.String())),
})
export type Health = Static<typeof Health>
const _Health: Same<Health, components['schemas']['Health']> = true
void _Health

export const TenantKey = Type.Object({
  id: Type.String({ format: 'uuid' }),
  prefix: Type.String({ description: 'The first characters of the key' }),
  label: Type.String(),
  createdAt: Type.String({ format: 'date-time' }),
  lastUsedAt: Type.Optional(Type.String({ format: 'date-time' })),
  expiresAt: Type.Optional(Type.String({ format: 'date-time' })),
  revokedAt: Type.Optional(Type.String({ format: 'date-time' })),
  status: Type.Union([Type.Literal('live'), Type.Literal('expired'), Type.Literal('revoked')]),
})
export type TenantKey = Static<typeof TenantKey>
const _TenantKey: Same<TenantKey, components['schemas']['TenantKey']> = true
void _TenantKey

export const IssuedTenantKey = Type.Intersect([
  TenantKey,
  Type.Object({
    secret: Type.String({ description: 'Shown once.' }),
  }),
])
export type IssuedTenantKey = Static<typeof IssuedTenantKey>
const _IssuedTenantKey: Same<IssuedTenantKey, components['schemas']['IssuedTenantKey']> = true
void _IssuedTenantKey

export const Problem = Type.Object(
  {
    type: Type.String({ description: 'urn:dispute-engine:error:<code>' }),
    title: Type.String(),
    status: Type.Integer(),
    detail: Type.Optional(Type.String()),
    instance: Type.Optional(Type.String()),
    code: ErrorCode,
    retryable: Type.Boolean({
      description:
        'true when the same request (unavailable) or a re-evaluated one (concurrent-update) can succeed later.',
    }),
    retryAfterSeconds: Type.Optional(Type.Integer()),
    requestId: Type.String(),
    allowedEvents: Type.Optional(
      Type.Array(DisputeEvent, {
        description: 'Present on invalid-transition; what the dispute accepts right now.',
      }),
    ),
    errors: Type.Optional(
      Type.Array(FieldError, {
        description:
          'Field-level failures for contract-violation and business validation; field is a JSON pointer into the body, or query.x / header.x / path.x.',
      }),
    ),
  },
  {
    description:
      'RFC 9457 problem details. Every field is safe to show to a user; diagnostics stay in server logs under requestId.',
  },
)
export type Problem = Static<typeof Problem>
const _Problem: Same<Problem, components['schemas']['Problem']> = true
void _Problem

export const SessionBlob = Type.Object(
  {
    tenantId: Type.Optional(
      Type.String({
        description: 'Set once the session belongs to a signed-in analyst; informational.',
        format: 'uuid',
      }),
    ),
    subject: Type.Optional(Type.String({ description: "The analyst's subject at the realm" })),
    sid: Type.Optional(Type.String({ description: "The realm's session id from the ID token" })),
    ciphertext: Type.String({ format: 'byte', maxLength: 16384 }),
    expiresAt: Type.String({ format: 'date-time' }),
  },
  {
    description: 'Ciphertext the frontend server produced; the API stores it without being able to read it.',
  },
)
export type SessionBlob = Static<typeof SessionBlob>
const _SessionBlob: Same<SessionBlob, components['schemas']['SessionBlob']> = true
void _SessionBlob

export const TenantSummary = Type.Object({
  id: Type.String({ format: 'uuid' }),
  slug: Type.String(),
  name: Type.String(),
  issuer: Type.Optional(Type.String({ description: "OIDC issuer of the tenant's realm." })),
  emailDomains: Type.Array(Type.String(), {
    description: 'Work-email domains that map to this tenant at sign-in.',
  }),
})
export type TenantSummary = Static<typeof TenantSummary>
const _TenantSummary: Same<TenantSummary, components['schemas']['TenantSummary']> = true
void _TenantSummary
