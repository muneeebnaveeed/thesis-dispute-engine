// Generated from docs/api/openapi.yaml by scripts/generate-api.ts. Do not edit.

import { Type, type Static } from '@sinclair/typebox'
import type { components } from './schema.gen'

// Each schema is checked both ways against the openapi-typescript type, so the two generated files cannot drift.
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : never) : never

export const AnswerType = Type.Union(
  [Type.Literal('YES_NO'), Type.Literal('DATE'), Type.Literal('TEXT'), Type.Literal('AMOUNT')],
  {
    description:
      'How an answer is validated; every answer travels as a string ("yes"/"no", YYYY-MM-DD, free text, decimal).',
  },
)
export type AnswerType = Static<typeof AnswerType>
const _AnswerType: Same<AnswerType, components['schemas']['AnswerType']> = true
void _AnswerType

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

export const SuspenseSettlement = Type.Union([Type.Literal('RECOVERED'), Type.Literal('WRITTEN_OFF')])
export type SuspenseSettlement = Static<typeof SuspenseSettlement>
const _SuspenseSettlement: Same<SuspenseSettlement, components['schemas']['SuspenseSettlement']> = true
void _SuspenseSettlement

export const ApplyEventRequest = Type.Object({
  event: DisputeEvent,
  actor: Type.Optional(Type.String({ default: 'system' })),
  payload: Type.Optional(
    Type.Object(
      {
        liability: Type.Optional(Type.String({ example: '50.00' })),
        settlement: Type.Optional(SuspenseSettlement),
        answers: Type.Optional(Type.Record(Type.String(), Type.String())),
        riskOverride: Type.Optional(
          Type.String({
            description: 'Why a credit goes out despite a HIGH risk score; recorded on the event.',
          }),
        ),
      },
      {
        description:
          "Event-specific facts, stored verbatim on the log entry. Two are read by the ledger: on ISSUE_REFUND, `liability` is the amount the customer bears (a decimal string, capped by the regime, refused with invalid-liability); on CLOSE, `settlement` says how an outstanding advance clears (RECOVERED or WRITTEN_OFF; the regime's default when absent; refused with invalid-settlement); on RECEIVE_QUESTIONNAIRE, `answers` maps question ids to answers and is validated against the questions that were sent (refused with invalid-answers, one error per question). On ISSUE_REFUND for a dispute whose latest risk tier is HIGH, `riskOverride` must carry the analyst's justification or the credit is refused with risk-hold.",
      },
    ),
  ),
})
export type ApplyEventRequest = Static<typeof ApplyEventRequest>
const _ApplyEventRequest: Same<ApplyEventRequest, components['schemas']['ApplyEventRequest']> = true
void _ApplyEventRequest

export const Attachment = Type.Object({
  id: Type.String({ format: 'uuid' }),
  filename: Type.String(),
  contentType: Type.String(),
  size: Type.Integer(),
})
export type Attachment = Static<typeof Attachment>
const _Attachment: Same<Attachment, components['schemas']['Attachment']> = true
void _Attachment

export const Balances = Type.Object(
  {
    customer: Type.String(),
    suspense: Type.String(),
    recovery: Type.String(),
    loss: Type.String(),
  },
  {
    description:
      "Running totals per account; customer from the customer's side (positive means credited), the rest from the bank's.",
  },
)
export type Balances = Static<typeof Balances>
const _Balances: Same<Balances, components['schemas']['Balances']> = true
void _Balances

export const Channel = Type.Union([Type.Literal('EMAIL'), Type.Literal('LETTER')], {
  description:
    'EMAIL goes out through the mail relay; LETTER is a printable document, ready the moment it exists.',
})
export type Channel = Static<typeof Channel>
const _Channel: Same<Channel, components['schemas']['Channel']> = true
void _Channel

export const NoticeKind = Type.Union(
  [
    Type.Literal('ACKNOWLEDGEMENT'),
    Type.Literal('QUESTIONNAIRE'),
    Type.Literal('PROVISIONAL_CREDIT'),
    Type.Literal('REFUND'),
    Type.Literal('REVERSAL'),
    Type.Literal('RESOLUTION'),
    Type.Literal('REQUEST_FOR_INFORMATION'),
    Type.Literal('STATUS_UPDATE'),
    Type.Literal('DOCUMENTS_RECEIVED'),
    Type.Literal('CUSTOM'),
  ],
  { description: 'The first six the engine sends on its own; the rest an analyst composes from a template.' },
)
export type NoticeKind = Static<typeof NoticeKind>
const _NoticeKind: Same<NoticeKind, components['schemas']['NoticeKind']> = true
void _NoticeKind

export const ComposeEmailRequest = Type.Object({
  template: NoticeKind,
  fields: Type.Record(Type.String(), Type.String(), {
    description: 'Field id to value; MULTISELECT values are comma-separated option keys.',
  }),
  attachments: Type.Optional(
    Type.Array(Type.String({ format: 'uuid' }), {
      description:
        'Draft attachment ids uploaded for this dispute, at most 3; they travel with the email and are listed on the letter.',
    }),
  ),
})
export type ComposeEmailRequest = Static<typeof ComposeEmailRequest>
const _ComposeEmailRequest: Same<ComposeEmailRequest, components['schemas']['ComposeEmailRequest']> = true
void _ComposeEmailRequest

export const CoreReceipt = Type.Object({
  rrn: Type.String({
    description: 'Retrieval reference number (ISO 8583 DE37); empty when the tenant has no core.',
  }),
  responseCode: Type.String({ description: 'ISO 8583 DE39; 00 approved' }),
  latencyMs: Type.Integer({ format: 'int64' }),
})
export type CoreReceipt = Static<typeof CoreReceipt>
const _CoreReceipt: Same<CoreReceipt, components['schemas']['CoreReceipt']> = true
void _CoreReceipt

export const DisputeReason = Type.Union([
  Type.Literal('UNAUTHORISED'),
  Type.Literal('NOT_RECEIVED'),
  Type.Literal('DUPLICATE'),
  Type.Literal('AMOUNT_DIFFERS'),
])
export type DisputeReason = Static<typeof DisputeReason>
const _DisputeReason: Same<DisputeReason, components['schemas']['DisputeReason']> = true
void _DisputeReason

export const CreateDisputeRequest = Type.Object({
  transactionId: Type.String({ format: 'uuid' }),
  reason: Type.Optional(DisputeReason),
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

export const DeadlineKind = Type.Union([
  Type.Literal('REFUND'),
  Type.Literal('ACKNOWLEDGE'),
  Type.Literal('RESOLUTION'),
])
export type DeadlineKind = Static<typeof DeadlineKind>
const _DeadlineKind: Same<DeadlineKind, components['schemas']['DeadlineKind']> = true
void _DeadlineKind

export const DeadlineStatus = Type.Union(
  [
    Type.Literal('RUNNING'),
    Type.Literal('MET'),
    Type.Literal('LATE'),
    Type.Literal('BREACHED'),
    Type.Literal('VOID'),
  ],
  {
    description:
      'RUNNING and BREACHED are open; MET and LATE are satisfied (before or after due); VOID no longer applies.',
  },
)
export type DeadlineStatus = Static<typeof DeadlineStatus>
const _DeadlineStatus: Same<DeadlineStatus, components['schemas']['DeadlineStatus']> = true
void _DeadlineStatus

export const Deadline = Type.Object({
  kind: DeadlineKind,
  cycle: Type.Integer({
    description: '0 for the clocks started at opening; the appeal number for a restarted clock.',
  }),
  startedAt: Type.String({ format: 'date-time' }),
  dueAt: Type.String({
    description: "End of the last permitted day in the tenant's calendar.",
    format: 'date-time',
  }),
  metAt: Type.Optional(Type.String({ format: 'date-time' })),
  status: DeadlineStatus,
  basis: Type.String({ description: 'The regulatory provision the clock comes from.' }),
})
export type Deadline = Static<typeof Deadline>
const _Deadline: Same<Deadline, components['schemas']['Deadline']> = true
void _Deadline

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

export const PostingKind = Type.Union([
  Type.Literal('PROVISIONAL_CREDIT'),
  Type.Literal('FAST_REFUND'),
  Type.Literal('NQA_REFUND'),
  Type.Literal('PROVISIONAL_CREDIT_REVERSAL'),
  Type.Literal('RECOVERY'),
  Type.Literal('WRITE_OFF'),
])
export type PostingKind = Static<typeof PostingKind>
const _PostingKind: Same<PostingKind, components['schemas']['PostingKind']> = true
void _PostingKind

export const LedgerAccount = Type.Union(
  [Type.Literal('CUSTOMER'), Type.Literal('SUSPENSE'), Type.Literal('RECOVERY'), Type.Literal('LOSS')],
  {
    description:
      "CUSTOMER is the disputing customer's account; SUSPENSE holds what the bank advanced; RECOVERY and LOSS clear it.",
  },
)
export type LedgerAccount = Static<typeof LedgerAccount>
const _LedgerAccount: Same<LedgerAccount, components['schemas']['LedgerAccount']> = true
void _LedgerAccount

export const LedgerEntry = Type.Object(
  {
    seq: Type.Integer({ description: 'The event that caused it.' }),
    kind: PostingKind,
    debit: LedgerAccount,
    credit: LedgerAccount,
    amount: Type.String({ description: "Decimal as a string at the currency's minor unit; never a float." }),
    currency: Type.String({ minLength: 3, maxLength: 3 }),
    reference: Type.String({ description: 'Unique per posting; what the banking core is told.' }),
    postedAt: Type.String({ format: 'date-time' }),
    core: Type.Optional(CoreReceipt),
  },
  {
    description:
      'One double-entry movement the engine instructed; amount leaves the credit account and lands in the debit account.',
  },
)
export type LedgerEntry = Static<typeof LedgerEntry>
const _LedgerEntry: Same<LedgerEntry, components['schemas']['LedgerEntry']> = true
void _LedgerEntry

export const Question = Type.Object({
  id: Type.String(),
  text: Type.String(),
  type: AnswerType,
  required: Type.Boolean(),
})
export type Question = Static<typeof Question>
const _Question: Same<Question, components['schemas']['Question']> = true
void _Question

export const Questionnaire = Type.Object({
  reason: DisputeReason,
  questions: Type.Array(Question, {
    description:
      'The questions as they were asked; a later change to the set never affects a sent questionnaire.',
  }),
  answers: Type.Optional(Type.Record(Type.String(), Type.String())),
  inconsistencies: Type.Array(Type.String(), {
    description: 'Contradictions found between the answers; empty until received or when none.',
  }),
  sentAt: Type.String({ format: 'date-time' }),
  receivedAt: Type.Optional(Type.String({ format: 'date-time' })),
})
export type Questionnaire = Static<typeof Questionnaire>
const _Questionnaire: Same<Questionnaire, components['schemas']['Questionnaire']> = true
void _Questionnaire

export const Notice = Type.Object({
  id: Type.Integer({ format: 'int64' }),
  seq: Type.Integer({ description: 'The event that caused it.' }),
  kind: NoticeKind,
  channel: Channel,
  recipient: Type.String(),
  subject: Type.String(),
  createdAt: Type.String({ format: 'date-time' }),
  sentAt: Type.Optional(
    Type.String({ description: 'Absent while an email waits in the outbox.', format: 'date-time' }),
  ),
  error: Type.Optional(Type.String({ description: 'The last delivery failure' })),
  actor: Type.Optional(
    Type.String({ description: "The analyst who composed it; absent for the engine's own notices." }),
  ),
  resendOf: Type.Optional(Type.Integer({ description: 'The notice this one repeats', format: 'int64' })),
  attachments: Type.Array(Attachment),
})
export type Notice = Static<typeof Notice>
const _Notice: Same<Notice, components['schemas']['Notice']> = true
void _Notice

export const RiskTier = Type.Union([Type.Literal('LOW'), Type.Literal('MEDIUM'), Type.Literal('HIGH')], {
  description:
    'LOW proceeds; MEDIUM is flagged for the analyst; HIGH holds credits until a justification is recorded.',
})
export type RiskTier = Static<typeof RiskTier>
const _RiskTier: Same<RiskTier, components['schemas']['RiskTier']> = true
void _RiskTier

export const RiskSignal = Type.Object({
  name: Type.String(),
  weight: Type.Integer({ description: 'The most this signal can add.' }),
  points: Type.Integer(),
  detail: Type.String({ description: 'Why it scored what it did' }),
})
export type RiskSignal = Static<typeof RiskSignal>
const _RiskSignal: Same<RiskSignal, components['schemas']['RiskSignal']> = true
void _RiskSignal

export const Risk = Type.Object({
  score: Type.Integer(),
  tier: RiskTier,
  signals: Type.Array(RiskSignal),
  assessedAt: Type.String({ format: 'date-time' }),
  history: Type.Array(
    Type.Object({
      seq: Type.Integer(),
      score: Type.Integer(),
      tier: RiskTier,
      assessedAt: Type.String({ format: 'date-time' }),
    }),
    { description: 'Earlier assessments, oldest first; the score moves when the questionnaire arrives.' },
  ),
})
export type Risk = Static<typeof Risk>
const _Risk: Same<Risk, components['schemas']['Risk']> = true
void _Risk

export const Dispute = Type.Object({
  id: Type.String({ format: 'uuid' }),
  regime: Regime,
  reason: DisputeReason,
  state: DisputeState,
  appeals: Type.Integer(),
  version: Type.Integer({ format: 'int64' }),
  transactionId: Type.String({ format: 'uuid' }),
  accountId: Type.String({ format: 'uuid' }),
  disputedAmount: Type.String({
    description: "Decimal as a string at the currency's minor unit; never a float.",
    example: '125.40',
  }),
  currency: Type.String({ minLength: 3, maxLength: 3 }),
  openedAt: Type.String({ format: 'date-time' }),
  updatedAt: Type.String({ format: 'date-time' }),
  allowedEvents: Type.Array(DisputeEvent),
  events: Type.Array(LoggedEvent),
  deadlines: Type.Array(Deadline, {
    description:
      'Every regulatory clock the regime started for this dispute, opening clocks first, then per appeal.',
  }),
  ledger: Type.Array(LedgerEntry, {
    description: 'Every movement the engine instructed on this dispute, in posting order.',
  }),
  balances: Balances,
  questionnaire: Type.Optional(Questionnaire),
  notices: Type.Array(Notice, {
    description: 'Every communication owed to the customer so far, in the order it arose.',
  }),
  risk: Type.Optional(Risk),
})
export type Dispute = Static<typeof Dispute>
const _Dispute: Same<Dispute, components['schemas']['Dispute']> = true
void _Dispute

export const DisputeSummary = Type.Object({
  id: Type.String({ format: 'uuid' }),
  regime: Regime,
  reason: DisputeReason,
  state: DisputeState,
  transactionId: Type.String({ format: 'uuid' }),
  disputedAmount: Type.String({
    description: "Decimal as a string at the currency's minor unit; never a float.",
  }),
  currency: Type.String({ minLength: 3, maxLength: 3 }),
  openedAt: Type.String({ format: 'date-time' }),
  updatedAt: Type.String({ format: 'date-time' }),
  nextDeadline: Type.Optional(Deadline),
  riskTier: Type.Optional(RiskTier),
  riskScore: Type.Optional(Type.Integer()),
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

export const DisputeReasonSuggestion = Type.Object(
  {
    reason: Type.Optional(DisputeReason),
    probability: Type.Optional(
      Type.Number({
        description: 'How sure the model was, recorded with the dispute when the analyst accepts it.',
        format: 'double',
        minimum: 0,
        maximum: 1,
      }),
    ),
  },
  { additionalProperties: false },
)
export type DisputeReasonSuggestion = Static<typeof DisputeReasonSuggestion>
const _DisputeReasonSuggestion: Same<
  DisputeReasonSuggestion,
  components['schemas']['DisputeReasonSuggestion']
> = true
void _DisputeReasonSuggestion

export const DisputeReasonSuggestionRequest = Type.Object(
  {
    description: Type.String({
      description: 'What the customer said, in any language.',
      minLength: 1,
      maxLength: 2000,
    }),
  },
  { additionalProperties: false },
)
export type DisputeReasonSuggestionRequest = Static<typeof DisputeReasonSuggestionRequest>
const _DisputeReasonSuggestionRequest: Same<
  DisputeReasonSuggestionRequest,
  components['schemas']['DisputeReasonSuggestionRequest']
> = true
void _DisputeReasonSuggestionRequest

export const FieldType = Type.Union(
  [
    Type.Literal('TEXT'),
    Type.Literal('TEXTAREA'),
    Type.Literal('NUMBER'),
    Type.Literal('DATE'),
    Type.Literal('SELECT'),
    Type.Literal('MULTISELECT'),
  ],
  {
    description:
      'TEXT and TEXTAREA are free text; NUMBER a whole number; DATE YYYY-MM-DD; SELECT one option key; MULTISELECT comma-separated option keys.',
  },
)
export type FieldType = Static<typeof FieldType>
const _FieldType: Same<FieldType, components['schemas']['FieldType']> = true
void _FieldType

export const TemplateOption = Type.Object({
  key: Type.String(),
  label: Type.String({ description: 'What the analyst sees.' }),
  text: Type.String({ description: 'What the customer reads.' }),
})
export type TemplateOption = Static<typeof TemplateOption>
const _TemplateOption: Same<TemplateOption, components['schemas']['TemplateOption']> = true
void _TemplateOption

export const TemplateField = Type.Object({
  id: Type.String(),
  label: Type.String(),
  type: FieldType,
  required: Type.Boolean(),
  options: Type.Optional(Type.Array(TemplateOption)),
  list: Type.Optional(Type.Boolean({ description: 'A TEXTAREA whose lines are items' })),
  default: Type.Optional(Type.String()),
  min: Type.Optional(Type.Integer()),
  max: Type.Optional(Type.Integer()),
})
export type TemplateField = Static<typeof TemplateField>
const _TemplateField: Same<TemplateField, components['schemas']['TemplateField']> = true
void _TemplateField

export const EmailTemplate = Type.Object({
  kind: NoticeKind,
  label: Type.String(),
  description: Type.String(),
  letter: Type.Boolean({
    description: 'Also goes out as a letter where the regime requires written notices.',
  }),
  fields: Type.Array(TemplateField),
  subject: Type.String({ description: 'Facts filled in; fields left as {{id}}.' }),
  paragraphs: Type.Array(Type.String(), {
    description:
      'Facts filled in; fields left as {{id}}; {{#id}}...{{/id}} appears only when the field has a value; a paragraph that is only a list field renders as bullet lines; empty paragraphs are dropped.',
  }),
})
export type EmailTemplate = Static<typeof EmailTemplate>
const _EmailTemplate: Same<EmailTemplate, components['schemas']['EmailTemplate']> = true
void _EmailTemplate

export const EmailTemplates = Type.Object({
  templates: Type.Array(EmailTemplate),
  facts: Type.Object(
    {
      customer: Type.String(),
      bank: Type.String(),
      amount: Type.String(),
      merchant: Type.String(),
      dispute: Type.String(),
      today: Type.String({ format: 'date' }),
    },
    { description: "The engine-known values, for the preview's greeting and letterhead." },
  ),
})
export type EmailTemplates = Static<typeof EmailTemplates>
const _EmailTemplates: Same<EmailTemplates, components['schemas']['EmailTemplates']> = true
void _EmailTemplates

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
    Type.Literal('invalid-liability'),
    Type.Literal('invalid-settlement'),
    Type.Literal('core-declined'),
    Type.Literal('unknown-reason'),
    Type.Literal('invalid-answers'),
    Type.Literal('risk-hold'),
    Type.Literal('invalid-fields'),
    Type.Literal('unknown-template'),
    Type.Literal('not-resendable'),
    Type.Literal('attachment-refused'),
    Type.Literal('attachment-unknown'),
    Type.Literal('invalid-template-override'),
    Type.Literal('concurrent-update'),
    Type.Literal('idempotency-key-reuse'),
    Type.Literal('image-refused'),
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

export const NoticeDocument = Type.Object({
  id: Type.Integer({ format: 'int64' }),
  kind: NoticeKind,
  channel: Channel,
  recipient: Type.String(),
  bank: Type.String(),
  date: Type.String({ format: 'date-time' }),
  subject: Type.String(),
  greeting: Type.String(),
  paragraphs: Type.Array(Type.String()),
  closing: Type.String(),
  basis: Type.Optional(Type.String({ description: 'The provision the notice satisfies' })),
  sentAt: Type.Optional(Type.String({ format: 'date-time' })),
})
export type NoticeDocument = Static<typeof NoticeDocument>
const _NoticeDocument: Same<NoticeDocument, components['schemas']['NoticeDocument']> = true
void _NoticeDocument

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

export const QuestionnaireSuggestion = Type.Object(
  {
    answers: Type.Record(
      Type.String(),
      Type.Object(
        {
          value: Type.Union([Type.Literal('yes'), Type.Literal('no')]),
          probability: Type.Number({ format: 'double', minimum: 0, maximum: 1 }),
        },
        { additionalProperties: false },
      ),
      {
        description:
          'Question id to the answer proposed for it. A question the model was unsure about is absent, as is every question that is not a yes or no.\n',
      },
    ),
  },
  { additionalProperties: false },
)
export type QuestionnaireSuggestion = Static<typeof QuestionnaireSuggestion>
const _QuestionnaireSuggestion: Same<
  QuestionnaireSuggestion,
  components['schemas']['QuestionnaireSuggestion']
> = true
void _QuestionnaireSuggestion

export const QuestionnaireSuggestionRequest = Type.Object(
  {
    reply: Type.String({
      description: 'What the customer wrote back, in any language.',
      minLength: 1,
      maxLength: 4000,
    }),
  },
  { additionalProperties: false },
)
export type QuestionnaireSuggestionRequest = Static<typeof QuestionnaireSuggestionRequest>
const _QuestionnaireSuggestionRequest: Same<
  QuestionnaireSuggestionRequest,
  components['schemas']['QuestionnaireSuggestionRequest']
> = true
void _QuestionnaireSuggestionRequest

export const SearchFilterSuggestion = Type.Object(
  {
    state: Type.Optional(DisputeState),
    reason: Type.Optional(DisputeReason),
    overdue: Type.Optional(
      Type.Boolean({ description: 'Present only when the sentence asks for disputes past a deadline.' }),
    ),
  },
  { additionalProperties: false },
)
export type SearchFilterSuggestion = Static<typeof SearchFilterSuggestion>
const _SearchFilterSuggestion: Same<SearchFilterSuggestion, components['schemas']['SearchFilterSuggestion']> =
  true
void _SearchFilterSuggestion

export const SearchFilterSuggestionRequest = Type.Object(
  {
    query: Type.String({
      description: 'What the analyst typed, in any language.',
      minLength: 1,
      maxLength: 300,
    }),
  },
  { additionalProperties: false },
)
export type SearchFilterSuggestionRequest = Static<typeof SearchFilterSuggestionRequest>
const _SearchFilterSuggestionRequest: Same<
  SearchFilterSuggestionRequest,
  components['schemas']['SearchFilterSuggestionRequest']
> = true
void _SearchFilterSuggestionRequest

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

export const TemplateOverride = Type.Object(
  {
    label: Type.Optional(Type.String()),
    description: Type.Optional(Type.String()),
    subject: Type.Optional(Type.String()),
    paragraphs: Type.Optional(Type.Array(Type.String())),
    optionTexts: Type.Optional(
      Type.Record(Type.String(), Type.String(), {
        description: 'Customer-facing text per option, keyed "fieldId.optionKey".',
      }),
    ),
  },
  { description: "A tenant's wording; empty members leave the base as it is." },
)
export type TemplateOverride = Static<typeof TemplateOverride>
const _TemplateOverride: Same<TemplateOverride, components['schemas']['TemplateOverride']> = true
void _TemplateOverride

export const TemplateSetting = Type.Object({
  base: EmailTemplate,
  override: Type.Optional(TemplateOverride),
  effective: EmailTemplate,
  updatedBy: Type.Optional(Type.String()),
  updatedAt: Type.Optional(Type.String({ format: 'date-time' })),
})
export type TemplateSetting = Static<typeof TemplateSetting>
const _TemplateSetting: Same<TemplateSetting, components['schemas']['TemplateSetting']> = true
void _TemplateSetting

export const Tenant = Type.Object({
  name: Type.String(),
  hasLogo: Type.Boolean({ description: 'Whether GetTenantLogo will answer with an image.' }),
  logoUpdatedAt: Type.Optional(Type.String({ format: 'date-time' })),
})
export type Tenant = Static<typeof Tenant>
const _Tenant: Same<Tenant, components['schemas']['Tenant']> = true
void _Tenant

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
