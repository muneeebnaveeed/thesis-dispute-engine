// Generated from docs/api/openapi.yaml by scripts/generate-api.ts. Do not edit.
export type paths = {
  '/healthz': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** Liveness */
    get: operations['getHealthz']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/readyz': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** Readiness (database reachable) */
    get: operations['getReadyz']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** The tenant's disputes, newest first */
    get: operations['listDisputes']
    put?: never
    /** Open a dispute against a transaction */
    post: operations['createDispute']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** Current state and full event log */
    get: operations['getDispute']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/events': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /** Apply a lifecycle event */
    post: operations['applyDisputeEvent']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/email-templates': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /**
     * The emails an analyst may compose for this dispute
     * @description Each template comes with its form and its words, the facts the engine knows already filled in and the analyst's fields left as {{placeholders}}, so the browser can preview as the analyst types. The server renders the final message on send from the same template.
     */
    get: operations['listEmailTemplates']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/notices': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /**
     * Compose and send an email from a template
     * @description Validates the fields against the template (invalid-fields, one error per field), composes the message server-side, queues the email and, where the regime requires writing and the template is a letter, the letter too. Analysts only; a tenant key cannot write to customers.
     */
    post: operations['composeEmail']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/attachments': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /**
     * Upload a file to attach to an email
     * @description A draft until an email claims it through ComposeEmailRequest.attachments; drafts older than a day are swept. PDF, PNG or JPEG, at most 5 MB. Analysts only.
     */
    post: operations['uploadAttachment']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/attachments/{attachmentId}': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** Download an attachment */
    get: operations['getAttachment']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/notices/{noticeId}/resend': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /**
     * Send an email again
     * @description Queues a new email with the original's words to the customer's current address, chained to the original and authored by the analyst. Only an email that has been sent can be resent (not-resendable otherwise); an email still in the outbox is retried by the engine on its own. Analysts only.
     */
    post: operations['resendNotice']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/notices/{noticeId}': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /**
     * One communication to the customer, as composed
     * @description The paragraphs as sent or as ready to print; the workbench renders letters from this.
     */
    get: operations['getNotice']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/tenant-templates': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /**
     * The tenant's wording for each analyst email kind
     * @description Tenant admins only. Each kind comes with the base template, the tenant's override if any, and the result analysts see.
     */
    get: operations['listTenantTemplates']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/tenant-templates/{kind}': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    /**
     * Set the tenant's wording for one kind
     * @description Words only: label, description, subject, paragraphs and option texts by "fieldId.optionKey". The form (field ids and types) stays the base's. Placeholders must name the form's fields or the engine's facts; refused with invalid-template-override, one error per problem.
     */
    put: operations['putTenantTemplate']
    post?: never
    /** Revert one kind to the base wording */
    delete: operations['deleteTenantTemplate']
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/suggestions/search-filters': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /**
     * Read an analyst's sentence as search filters
     * @description A convenience, never a decision. The filters come back only when the model is sure enough, so an empty answer is the normal case and never an error; the analyst sees the filters and can change them before searching. Nothing is stored.
     */
    post: operations['suggestSearchFilters']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/disputes/{disputeId}/suggestions/questionnaire': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /**
     * Read a customer's reply as questionnaire answers
     * @description A convenience, never a decision. Only the yes and no questions of this dispute's questionnaire are answered, and only those the model was sure about; dates and free text are left to the analyst, who confirms everything before any answer is recorded. The reply is read and not stored.
     */
    post: operations['suggestQuestionnaireAnswers']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/suggestions/dispute-reason': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    /**
     * Read what a customer wrote as a dispute reason
     * @description A convenience, never a decision. The reason comes back only when the model is sure enough, and the analyst confirms it before a dispute is opened; the description is read and not stored, so the record still holds only what a person chose.
     */
    post: operations['suggestDisputeReason']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/tenant': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** What the workbench shows about the signed-in analyst's organisation */
    get: operations['getTenant']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/tenant/logo': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** The organisation's logo */
    get: operations['getTenantLogo']
    /**
     * Replace the organisation's logo
     * @description PNG, JPEG or WebP, at most 256 kB. Tenant admins only.
     */
    put: operations['putTenantLogo']
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/me/avatar': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** The signed-in analyst's picture */
    get: operations['getMyAvatar']
    /**
     * Replace the signed-in analyst's picture
     * @description PNG, JPEG or WebP, at most 256 kB. Analysts only; a tenant key has no person behind it.
     */
    put: operations['putMyAvatar']
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/tenant-keys': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /**
     * The tenant's own keys, live and revoked
     * @description Analysts with the tenant-admin role only; a tenant key cannot inspect keys.
     */
    get: operations['listTenantKeys']
    put?: never
    /**
     * Issue a key for one of the tenant's systems
     * @description The secret is in this response only; the server keeps its hash and prefix.
     */
    post: operations['createTenantKey']
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/tenant-keys/{keyId}': {
    parameters: {
      query?: never
      header?: never
      path: {
        keyId: string
      }
      cookie?: never
    }
    get?: never
    put?: never
    post?: never
    /** Revoke a key; takes effect on the next request */
    delete: operations['revokeTenantKey']
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/internal/sessions/{sessionId}': {
    parameters: {
      query?: never
      header?: never
      path: {
        sessionId: string
      }
      cookie?: never
    }
    /** Fetch a session blob that has not expired */
    get: operations['getSession']
    /** Store or replace an opaque session blob */
    put: operations['putSession']
    post?: never
    /** Remove a session */
    delete: operations['deleteSession']
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/internal/sessions': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    get?: never
    put?: never
    post?: never
    /** End every session matching a realm session id or a tenant's subject */
    delete: operations['deleteSessions']
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
  '/internal/tenants': {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    /** Active tenants with what the sign-in page needs to find their realm */
    get: operations['listTenants']
    put?: never
    post?: never
    delete?: never
    options?: never
    head?: never
    patch?: never
    trace?: never
  }
}
export type webhooks = Record<string, never>
export type components = {
  schemas: {
    Health: {
      /** @enum {string} */
      status: 'ok' | 'unavailable'
      checks?: {
        [key: string]: string
      }
    }
    /**
     * @description Stable machine-readable failure identifier; branch on this, never on text.
     * @enum {string}
     */
    ErrorCode:
      | 'unauthenticated'
      | 'forbidden'
      | 'rate-limited'
      | 'malformed-request'
      | 'contract-violation'
      | 'not-found'
      | 'invalid-transition'
      | 'appeals-exhausted'
      | 'invalid-liability'
      | 'invalid-settlement'
      | 'core-declined'
      | 'unknown-reason'
      | 'invalid-answers'
      | 'risk-hold'
      | 'invalid-fields'
      | 'unknown-template'
      | 'not-resendable'
      | 'attachment-refused'
      | 'attachment-unknown'
      | 'invalid-template-override'
      | 'concurrent-update'
      | 'idempotency-key-reuse'
      | 'image-refused'
      | 'no-regime'
      | 'unknown-regime'
      | 'unavailable'
      | 'internal'
    /** @description RFC 9457 problem details. Every field is safe to show to a user; diagnostics stay in server logs under requestId. */
    Problem: {
      /** @description urn:dispute-engine:error:<code> */
      type: string
      title: string
      status: number
      detail?: string
      instance?: string
      code: components['schemas']['ErrorCode']
      /** @description true when the same request (unavailable) or a re-evaluated one (concurrent-update) can succeed later. */
      retryable: boolean
      retryAfterSeconds?: number
      requestId: string
      /** @description Present on invalid-transition; what the dispute accepts right now. */
      allowedEvents?: components['schemas']['DisputeEvent'][]
      /** @description Field-level failures for contract-violation and business validation; field is a JSON pointer into the body, or query.x / header.x / path.x. */
      errors?: components['schemas']['FieldError'][]
    }
    FieldError: {
      /** @example /transactionId */
      field: string
      message: string
    }
    Tenant: {
      name: string
      /** @description Whether GetTenantLogo will answer with an image. */
      hasLogo: boolean
      /** Format: date-time */
      logoUpdatedAt?: string
    }
    TenantKey: {
      /** Format: uuid */
      id: string
      /** @description The first characters of the key */
      prefix: string
      label: string
      /** Format: date-time */
      createdAt: string
      /** Format: date-time */
      lastUsedAt?: string
      /** Format: date-time */
      expiresAt?: string
      /** Format: date-time */
      revokedAt?: string
      /** @enum {string} */
      status: 'live' | 'expired' | 'revoked'
    }
    CreateTenantKeyRequest: {
      /** @description Which system will hold it. */
      label: string
      /**
       * Format: date-time
       * @description Optional; the key stops working after this.
       */
      expiresAt?: string
    }
    IssuedTenantKey: components['schemas']['TenantKey'] & {
      /** @description Shown once. */
      secret: string
    }
    TenantSummary: {
      /** Format: uuid */
      id: string
      slug: string
      name: string
      /** @description OIDC issuer of the tenant's realm. */
      issuer?: string
      /** @description Work-email domains that map to this tenant at sign-in. */
      emailDomains: string[]
    }
    /** @description Ciphertext the frontend server produced; the API stores it without being able to read it. */
    SessionBlob: {
      /**
       * Format: uuid
       * @description Set once the session belongs to a signed-in analyst; informational.
       */
      tenantId?: string
      /** @description The analyst's subject at the realm */
      subject?: string
      /** @description The realm's session id from the ID token */
      sid?: string
      /** Format: byte */
      ciphertext: string
      /** Format: date-time */
      expiresAt: string
    }
    /** @enum {string} */
    Regime: 'EU_SEPA_DIRECT_DEBIT' | 'EU_PSD2_CARD' | 'US_REG_E' | 'US_REG_Z'
    QuestionnaireSuggestionRequest: {
      /** @description What the customer wrote back, in any language. */
      reply: string
    }
    QuestionnaireSuggestion: {
      /** @description Question id to the answer proposed for it. A question the model was unsure about is absent, as is every question that is not a yes or no. */
      answers: {
        [key: string]: {
          /** @enum {string} */
          value: 'yes' | 'no'
          /** Format: double */
          probability: number
        }
      }
    }
    /** @description What a model proposed for a field, sent back when the dispute is opened so the record can say the analyst was shown a suggestion and what they did with it. It changes nothing about the dispute. */
    AcceptedSuggestion: {
      reason: components['schemas']['DisputeReason']
      /** Format: double */
      probability: number
    }
    DisputeReasonSuggestionRequest: {
      /** @description What the customer said, in any language. */
      description: string
    }
    /** @description An absent reason means the model was not sure enough to offer one. */
    DisputeReasonSuggestion: {
      reason?: components['schemas']['DisputeReason']
      /**
       * Format: double
       * @description How sure the model was, recorded with the dispute when the analyst accepts it.
       */
      probability?: number
    }
    SearchFilterSuggestionRequest: {
      /** @description What the analyst typed, in any language. */
      query: string
    }
    /** @description Every field is optional; an absent field is one the model was not sure enough about. */
    SearchFilterSuggestion: {
      state?: components['schemas']['DisputeState']
      reason?: components['schemas']['DisputeReason']
      /** @description Present only when the sentence asks for disputes past a deadline. */
      overdue?: boolean
    }
    /** @enum {string} */
    DisputeState:
      | 'INITIATED'
      | 'INVESTIGATING'
      | 'QUESTIONNAIRE_SENT'
      | 'QUESTIONNAIRE_RECEIVED'
      | 'PROVISIONAL_CREDIT_ISSUED'
      | 'FAST_REFUND_ISSUED'
      | 'SEPA_NQA_REFUND_ISSUED'
      | 'CHARGEBACK_FILED'
      | 'CHARGEBACK_ACKNOWLEDGED'
      | 'EVIDENCE_SUBMITTED'
      | 'CHARGEBACK_WON'
      | 'CHARGEBACK_LOST'
      | 'FINAL_CREDIT_ISSUED'
      | 'PROVISIONAL_CREDIT_REVERSED'
      | 'CLOSED'
    /** @enum {string} */
    DisputeEvent:
      | 'OPEN_INVESTIGATION'
      | 'SEND_QUESTIONNAIRE'
      | 'RECEIVE_QUESTIONNAIRE'
      | 'ISSUE_REFUND'
      | 'FILE_CHARGEBACK'
      | 'ACKNOWLEDGE_CHARGEBACK'
      | 'SUBMIT_EVIDENCE'
      | 'WIN_CHARGEBACK'
      | 'LOSE_CHARGEBACK'
      | 'ISSUE_FINAL_CREDIT'
      | 'REVERSE_PROVISIONAL_CREDIT'
      | 'CLOSE'
      | 'APPEAL'
    CreateDisputeRequest: {
      /** Format: uuid */
      transactionId: string
      /** @description Why the customer disputes it; selects the questionnaire. Defaults to UNAUTHORISED. */
      reason?: components['schemas']['DisputeReason']
      /**
       * @description Who opened it; defaults to customer.
       * @default customer
       */
      actor?: string
      /** @description What a model proposed for the reason, if the analyst was shown one. Recorded beside the reason they chose so acceptance can be measured; it never changes what is opened. */
      suggestion?: components['schemas']['AcceptedSuggestion']
    }
    /** @enum {string} */
    DisputeReason: 'UNAUTHORISED' | 'NOT_RECEIVED' | 'DUPLICATE' | 'AMOUNT_DIFFERS'
    ApplyEventRequest: {
      event: components['schemas']['DisputeEvent']
      /** @default system */
      actor?: string
      /** @description Event-specific facts, stored verbatim on the log entry. Two are read by the ledger: on ISSUE_REFUND, `liability` is the amount the customer bears (a decimal string, capped by the regime, refused with invalid-liability); on CLOSE, `settlement` says how an outstanding advance clears (RECOVERED or WRITTEN_OFF; the regime's default when absent; refused with invalid-settlement); on RECEIVE_QUESTIONNAIRE, `answers` maps question ids to answers and is validated against the questions that were sent (refused with invalid-answers, one error per question), and `suggestions` may carry what a model proposed for those answers, stored beside them and read only as an acceptance rate. On ISSUE_REFUND for a dispute whose latest risk tier is HIGH, `riskOverride` must carry the analyst's justification or the credit is refused with risk-hold. */
      payload?: {
        /** @example 50.00 */
        liability?: string
        settlement?: components['schemas']['SuspenseSettlement']
        answers?: {
          [key: string]: string
        }
        /** @description Why a credit goes out despite a HIGH risk score; recorded on the event. */
        riskOverride?: string
      } & {
        [key: string]: unknown
      }
    }
    /** @enum {string} */
    SuspenseSettlement: 'RECOVERED' | 'WRITTEN_OFF'
    /**
     * @description CUSTOMER is the disputing customer's account; SUSPENSE holds what the bank advanced; RECOVERY and LOSS clear it.
     * @enum {string}
     */
    LedgerAccount: 'CUSTOMER' | 'SUSPENSE' | 'RECOVERY' | 'LOSS'
    /** @enum {string} */
    PostingKind:
      | 'PROVISIONAL_CREDIT'
      | 'FAST_REFUND'
      | 'NQA_REFUND'
      | 'PROVISIONAL_CREDIT_REVERSAL'
      | 'RECOVERY'
      | 'WRITE_OFF'
    /** @description One double-entry movement the engine instructed; amount leaves the credit account and lands in the debit account. */
    LedgerEntry: {
      /** @description The event that caused it. */
      seq: number
      kind: components['schemas']['PostingKind']
      debit: components['schemas']['LedgerAccount']
      credit: components['schemas']['LedgerAccount']
      /** @description Decimal as a string at the currency's minor unit; never a float. */
      amount: string
      currency: string
      /** @description Unique per posting; what the banking core is told. */
      reference: string
      /** Format: date-time */
      postedAt: string
      /** @description The banking core's answer; present for postings that moved the customer's money. */
      core?: components['schemas']['CoreReceipt']
    }
    CoreReceipt: {
      /** @description Retrieval reference number (ISO 8583 DE37); empty when the tenant has no core. */
      rrn: string
      /** @description ISO 8583 DE39; 00 approved */
      responseCode: string
      /** Format: int64 */
      latencyMs: number
    }
    /** @description Running totals per account; customer from the customer's side (positive means credited), the rest from the bank's. */
    Balances: {
      customer: string
      suspense: string
      recovery: string
      loss: string
    }
    LoggedEvent: {
      seq: number
      /** @description A DisputeEvent, or OPENED for entry 1. */
      event: string
      fromState: string
      toState: components['schemas']['DisputeState']
      actor: string
      payload: {
        [key: string]: unknown
      }
      traceId?: string
      /** Format: date-time */
      occurredAt: string
    }
    DisputeSummary: {
      /** Format: uuid */
      id: string
      regime: components['schemas']['Regime']
      reason: components['schemas']['DisputeReason']
      state: components['schemas']['DisputeState']
      /** Format: uuid */
      transactionId: string
      /** @description Decimal as a string at the currency's minor unit; never a float. */
      disputedAmount: string
      currency: string
      /** Format: date-time */
      openedAt: string
      /** Format: date-time */
      updatedAt: string
      /** @description The open regulatory clock that runs out first; absent once every clock is settled. */
      nextDeadline?: components['schemas']['Deadline']
      riskTier?: components['schemas']['RiskTier']
      riskScore?: number
    }
    DisputePage: {
      items: components['schemas']['DisputeSummary'][]
      /** @description Present when there is another page. */
      nextCursor?: string
    }
    Dispute: {
      /** Format: uuid */
      id: string
      regime: components['schemas']['Regime']
      reason: components['schemas']['DisputeReason']
      state: components['schemas']['DisputeState']
      appeals: number
      /** Format: int64 */
      version: number
      /** Format: uuid */
      transactionId: string
      /** Format: uuid */
      accountId: string
      /**
       * @description Decimal as a string at the currency's minor unit; never a float.
       * @example 125.40
       */
      disputedAmount: string
      currency: string
      /** Format: date-time */
      openedAt: string
      /** Format: date-time */
      updatedAt: string
      allowedEvents: components['schemas']['DisputeEvent'][]
      events: components['schemas']['LoggedEvent'][]
      /** @description Every regulatory clock the regime started for this dispute, opening clocks first, then per appeal. */
      deadlines: components['schemas']['Deadline'][]
      /** @description Every movement the engine instructed on this dispute, in posting order. */
      ledger: components['schemas']['LedgerEntry'][]
      balances: components['schemas']['Balances']
      /** @description Present once SEND_QUESTIONNAIRE has been applied. */
      questionnaire?: components['schemas']['Questionnaire']
      /** @description Every communication owed to the customer so far, in the order it arose. */
      notices: components['schemas']['Notice'][]
      /** @description The latest fraud assessment; absent for disputes opened before scoring existed. */
      risk?: components['schemas']['Risk']
    }
    /**
     * @description LOW proceeds; MEDIUM is flagged for the analyst; HIGH holds credits until a justification is recorded.
     * @enum {string}
     */
    RiskTier: 'LOW' | 'MEDIUM' | 'HIGH'
    RiskSignal: {
      name: string
      /** @description The most this signal can add. */
      weight: number
      points: number
      /** @description Why it scored what it did */
      detail: string
    }
    Risk: {
      score: number
      tier: components['schemas']['RiskTier']
      signals: components['schemas']['RiskSignal'][]
      /** Format: date-time */
      assessedAt: string
      /** @description Earlier assessments, oldest first; the score moves when the questionnaire arrives. */
      history: {
        seq: number
        score: number
        tier: components['schemas']['RiskTier']
        /** Format: date-time */
        assessedAt: string
      }[]
    }
    /**
     * @description The first six the engine sends on its own; the rest an analyst composes from a template.
     * @enum {string}
     */
    NoticeKind:
      | 'ACKNOWLEDGEMENT'
      | 'QUESTIONNAIRE'
      | 'PROVISIONAL_CREDIT'
      | 'REFUND'
      | 'REVERSAL'
      | 'RESOLUTION'
      | 'REQUEST_FOR_INFORMATION'
      | 'STATUS_UPDATE'
      | 'DOCUMENTS_RECEIVED'
      | 'CUSTOM'
    /**
     * @description EMAIL goes out through the mail relay; LETTER is a printable document, ready the moment it exists.
     * @enum {string}
     */
    Channel: 'EMAIL' | 'LETTER'
    Notice: {
      /** Format: int64 */
      id: number
      /** @description The event that caused it. */
      seq: number
      kind: components['schemas']['NoticeKind']
      channel: components['schemas']['Channel']
      recipient: string
      subject: string
      /** Format: date-time */
      createdAt: string
      /**
       * Format: date-time
       * @description Absent while an email waits in the outbox.
       */
      sentAt?: string
      /** @description The last delivery failure */
      error?: string
      /** @description The analyst who composed it; absent for the engine's own notices. */
      actor?: string
      /**
       * Format: int64
       * @description The notice this one repeats
       */
      resendOf?: number
      attachments: components['schemas']['Attachment'][]
    }
    Attachment: {
      /** Format: uuid */
      id: string
      filename: string
      contentType: string
      size: number
    }
    /**
     * @description TEXT and TEXTAREA are free text; NUMBER a whole number; DATE YYYY-MM-DD; SELECT one option key; MULTISELECT comma-separated option keys.
     * @enum {string}
     */
    FieldType: 'TEXT' | 'TEXTAREA' | 'NUMBER' | 'DATE' | 'SELECT' | 'MULTISELECT'
    TemplateOption: {
      key: string
      /** @description What the analyst sees. */
      label: string
      /** @description What the customer reads. */
      text: string
    }
    TemplateField: {
      id: string
      label: string
      type: components['schemas']['FieldType']
      required: boolean
      options?: components['schemas']['TemplateOption'][]
      /** @description A TEXTAREA whose lines are items */
      list?: boolean
      default?: string
      min?: number
      max?: number
    }
    EmailTemplate: {
      kind: components['schemas']['NoticeKind']
      label: string
      description: string
      /** @description Also goes out as a letter where the regime requires written notices. */
      letter: boolean
      fields: components['schemas']['TemplateField'][]
      /** @description Facts filled in; fields left as {{id}}. */
      subject: string
      /** @description Facts filled in; fields left as {{id}}; {{#id}}...{{/id}} appears only when the field has a value; a paragraph that is only a list field renders as bullet lines; empty paragraphs are dropped. */
      paragraphs: string[]
    }
    EmailTemplates: {
      templates: components['schemas']['EmailTemplate'][]
      /** @description The engine-known values, for the preview's greeting and letterhead. */
      facts: {
        customer: string
        bank: string
        amount: string
        merchant: string
        dispute: string
        /** Format: date */
        today: string
      }
    }
    /** @description A tenant's wording; empty members leave the base as it is. */
    TemplateOverride: {
      label?: string
      description?: string
      subject?: string
      paragraphs?: string[]
      /** @description Customer-facing text per option, keyed "fieldId.optionKey". */
      optionTexts?: {
        [key: string]: string
      }
    }
    TemplateSetting: {
      base: components['schemas']['EmailTemplate']
      override?: components['schemas']['TemplateOverride']
      effective: components['schemas']['EmailTemplate']
      updatedBy?: string
      /** Format: date-time */
      updatedAt?: string
    }
    ComposeEmailRequest: {
      template: components['schemas']['NoticeKind']
      /** @description Field id to value; MULTISELECT values are comma-separated option keys. */
      fields: {
        [key: string]: string
      }
      /** @description Draft attachment ids uploaded for this dispute, at most 3; they travel with the email and are listed on the letter. */
      attachments?: string[]
    }
    NoticeDocument: {
      /** Format: int64 */
      id: number
      kind: components['schemas']['NoticeKind']
      channel: components['schemas']['Channel']
      recipient: string
      bank: string
      /** Format: date-time */
      date: string
      subject: string
      greeting: string
      paragraphs: string[]
      closing: string
      /** @description The provision the notice satisfies */
      basis?: string
      /** Format: date-time */
      sentAt?: string
    }
    /**
     * @description How an answer is validated; every answer travels as a string ("yes"/"no", YYYY-MM-DD, free text, decimal).
     * @enum {string}
     */
    AnswerType: 'YES_NO' | 'DATE' | 'TEXT' | 'AMOUNT'
    Question: {
      id: string
      text: string
      type: components['schemas']['AnswerType']
      required: boolean
    }
    Questionnaire: {
      reason: components['schemas']['DisputeReason']
      /** @description The questions as they were asked; a later change to the set never affects a sent questionnaire. */
      questions: components['schemas']['Question'][]
      answers?: {
        [key: string]: string
      }
      /** @description Contradictions found between the answers; empty until received or when none. */
      inconsistencies: string[]
      /** Format: date-time */
      sentAt: string
      /** Format: date-time */
      receivedAt?: string
    }
    /** @enum {string} */
    DeadlineKind: 'REFUND' | 'ACKNOWLEDGE' | 'RESOLUTION'
    /**
     * @description RUNNING and BREACHED are open; MET and LATE are satisfied (before or after due); VOID no longer applies.
     * @enum {string}
     */
    DeadlineStatus: 'RUNNING' | 'MET' | 'LATE' | 'BREACHED' | 'VOID'
    Deadline: {
      kind: components['schemas']['DeadlineKind']
      /** @description 0 for the clocks started at opening; the appeal number for a restarted clock. */
      cycle: number
      /** Format: date-time */
      startedAt: string
      /**
       * Format: date-time
       * @description End of the last permitted day in the tenant's calendar.
       */
      dueAt: string
      /** Format: date-time */
      metAt?: string
      status: components['schemas']['DeadlineStatus']
      /** @description The regulatory provision the clock comes from. */
      basis: string
    }
  }
  responses: {
    /** @description Malformed request. */
    BadRequest: {
      headers: {
        [name: string]: unknown
      }
      content: {
        'application/problem+json': components['schemas']['Problem']
      }
    }
    /** @description The tenant's request budget for the current minute is spent; retryAfterSeconds says when to try again. */
    TooManyRequests: {
      headers: {
        'Retry-After'?: number
        [name: string]: unknown
      }
      content: {
        'application/problem+json': components['schemas']['Problem']
      }
    }
    /** @description Missing, unknown or revoked tenant key. */
    Unauthorized: {
      headers: {
        [name: string]: unknown
      }
      content: {
        'application/problem+json': components['schemas']['Problem']
      }
    }
    /** @description The credential is valid but may not do this (an analyst without the tenant-admin role, or a tenant key on an analyst-only operation). */
    Forbidden: {
      headers: {
        [name: string]: unknown
      }
      content: {
        'application/problem+json': components['schemas']['Problem']
      }
    }
    /** @description No such resource. */
    NotFound: {
      headers: {
        [name: string]: unknown
      }
      content: {
        'application/problem+json': components['schemas']['Problem']
      }
    }
    /** @description Well-formed but not acceptable (idempotency key reused with a different body, no regime for the transaction, a liability or settlement the regime refuses, or a posting the banking core declined). */
    Unprocessable: {
      headers: {
        [name: string]: unknown
      }
      content: {
        'application/problem+json': components['schemas']['Problem']
      }
    }
  }
  parameters: {
    DisputeId: string
    /** @description Client-chosen key. Same key and body replays the original response; same key with a different body is 422. */
    IdempotencyKey: string
  }
  requestBodies: never
  headers: never
  pathItems: never
}
export type $defs = Record<string, never>
export interface operations {
  getHealthz: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Process is up. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Health']
        }
      }
    }
  }
  getReadyz: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Ready. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Health']
        }
      }
      /** @description A dependency is unavailable. */
      503: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Health']
        }
      }
    }
  }
  listDisputes: {
    parameters: {
      query?: {
        state?: components['schemas']['DisputeState']
        reason?: components['schemas']['DisputeReason']
        limit?: number
        /** @description Opaque; from the previous page's nextCursor. */
        cursor?: string
        /** @description Only disputes with at least one regulatory clock past its due time and still open. */
        overdue?: boolean
      }
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description One page. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['DisputePage']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      429: components['responses']['TooManyRequests']
    }
  }
  createDispute: {
    parameters: {
      query?: never
      header?: {
        /** @description Client-chosen key. Same key and body replays the original response; same key with a different body is 422. */
        'Idempotency-Key'?: components['parameters']['IdempotencyKey']
      }
      path?: never
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['CreateDisputeRequest']
      }
    }
    responses: {
      /** @description Dispute opened. Replays of the same Idempotency-Key return this same body. */
      201: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Dispute']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
    }
  }
  getDispute: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: components['parameters']['DisputeId']
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The dispute. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Dispute']
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  applyDisputeEvent: {
    parameters: {
      query?: never
      header?: {
        /** @description Client-chosen key. Same key and body replays the original response; same key with a different body is 422. */
        'Idempotency-Key'?: components['parameters']['IdempotencyKey']
      }
      path: {
        disputeId: components['parameters']['DisputeId']
      }
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['ApplyEventRequest']
      }
    }
    responses: {
      /** @description Event applied; the dispute after the transition. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Dispute']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      /** @description The event is not allowed from the current state, or a concurrent update won. */
      409: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/problem+json': components['schemas']['Problem']
        }
      }
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
      /** @description The tenant's banking core gave no answer in time; nothing was written. Repeat the request with the same Idempotency-Key: the core sees the same posting reference and will not move the money twice. */
      503: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/problem+json': components['schemas']['Problem']
        }
      }
    }
  }
  listEmailTemplates: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: string
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The catalogue. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['EmailTemplates']
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  composeEmail: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: string
      }
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['ComposeEmailRequest']
      }
    }
    responses: {
      /** @description Queued; the dispute with its communications. */
      201: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Dispute']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      404: components['responses']['NotFound']
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
    }
  }
  uploadAttachment: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: string
      }
      cookie?: never
    }
    requestBody: {
      content: {
        'multipart/form-data': {
          /** Format: binary */
          file: string
        }
      }
    }
    responses: {
      /** @description Stored as a draft. */
      201: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Attachment']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      404: components['responses']['NotFound']
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
    }
  }
  getAttachment: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: string
        attachmentId: string
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The file, with its own content type and a Content-Disposition naming it. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/octet-stream': string
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  resendNotice: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: string
        noticeId: number
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Queued; the dispute with its communications. */
      201: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Dispute']
        }
      }
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      404: components['responses']['NotFound']
      /** @description The notice is a letter or was never sent. */
      409: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/problem+json': components['schemas']['Problem']
        }
      }
      429: components['responses']['TooManyRequests']
    }
  }
  getNotice: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: string
        noticeId: number
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The notice. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['NoticeDocument']
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  listTenantTemplates: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description One entry per kind. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['TemplateSetting'][]
        }
      }
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      429: components['responses']['TooManyRequests']
    }
  }
  putTenantTemplate: {
    parameters: {
      query?: never
      header?: never
      path: {
        kind: components['schemas']['NoticeKind']
      }
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['TemplateOverride']
      }
    }
    responses: {
      /** @description Stored; the entry as analysts now see it. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['TemplateSetting']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
    }
  }
  deleteTenantTemplate: {
    parameters: {
      query?: never
      header?: never
      path: {
        kind: components['schemas']['NoticeKind']
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Reverted. */
      204: {
        headers: {
          [name: string]: unknown
        }
        content?: never
      }
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  suggestSearchFilters: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['SearchFilterSuggestionRequest']
      }
    }
    responses: {
      /** @description What the sentence appears to ask for, with any field the model was unsure of left out */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['SearchFilterSuggestion']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      429: components['responses']['TooManyRequests']
    }
  }
  suggestQuestionnaireAnswers: {
    parameters: {
      query?: never
      header?: never
      path: {
        disputeId: components['parameters']['DisputeId']
      }
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['QuestionnaireSuggestionRequest']
      }
    }
    responses: {
      /** @description The answers the reply appears to give, keyed by question id */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['QuestionnaireSuggestion']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  suggestDisputeReason: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['DisputeReasonSuggestionRequest']
      }
    }
    responses: {
      /** @description The reason the description appears to describe, absent when the model was unsure */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['DisputeReasonSuggestion']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      429: components['responses']['TooManyRequests']
    }
  }
  getTenant: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The tenant's name and whether it has a logo. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Tenant']
        }
      }
      401: components['responses']['Unauthorized']
      429: components['responses']['TooManyRequests']
    }
  }
  getTenantLogo: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The image. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'image/*': string
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  putTenantLogo: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody: {
      content: {
        'multipart/form-data': {
          /** Format: binary */
          file: string
        }
      }
    }
    responses: {
      /** @description Stored. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['Tenant']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
    }
  }
  getMyAvatar: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The image. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'image/*': string
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
      429: components['responses']['TooManyRequests']
    }
  }
  putMyAvatar: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody: {
      content: {
        'multipart/form-data': {
          /** Format: binary */
          file: string
        }
      }
    }
    responses: {
      /** @description Stored. */
      204: {
        headers: {
          [name: string]: unknown
        }
        content?: never
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      422: components['responses']['Unprocessable']
      429: components['responses']['TooManyRequests']
    }
  }
  listTenantKeys: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Keys, newest first. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['TenantKey'][]
        }
      }
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
    }
  }
  createTenantKey: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['CreateTenantKeyRequest']
      }
    }
    responses: {
      /** @description Issued. */
      201: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['IssuedTenantKey']
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
    }
  }
  revokeTenantKey: {
    parameters: {
      query?: never
      header?: never
      path: {
        keyId: string
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Revoked */
      204: {
        headers: {
          [name: string]: unknown
        }
        content?: never
      }
      401: components['responses']['Unauthorized']
      403: components['responses']['Forbidden']
      404: components['responses']['NotFound']
    }
  }
  getSession: {
    parameters: {
      query?: never
      header?: never
      path: {
        sessionId: string
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description The blob. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['SessionBlob']
        }
      }
      401: components['responses']['Unauthorized']
      404: components['responses']['NotFound']
    }
  }
  putSession: {
    parameters: {
      query?: never
      header?: never
      path: {
        sessionId: string
      }
      cookie?: never
    }
    requestBody: {
      content: {
        'application/json': components['schemas']['SessionBlob']
      }
    }
    responses: {
      /** @description Stored. */
      204: {
        headers: {
          [name: string]: unknown
        }
        content?: never
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
    }
  }
  deleteSession: {
    parameters: {
      query?: never
      header?: never
      path: {
        sessionId: string
      }
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Removed */
      204: {
        headers: {
          [name: string]: unknown
        }
        content?: never
      }
      401: components['responses']['Unauthorized']
    }
  }
  deleteSessions: {
    parameters: {
      query?: {
        sid?: string
        tenantId?: string
        subject?: string
      }
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description How many sessions were ended. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': {
            deleted: number
          }
        }
      }
      400: components['responses']['BadRequest']
      401: components['responses']['Unauthorized']
    }
  }
  listTenants: {
    parameters: {
      query?: never
      header?: never
      path?: never
      cookie?: never
    }
    requestBody?: never
    responses: {
      /** @description Active tenants. */
      200: {
        headers: {
          [name: string]: unknown
        }
        content: {
          'application/json': components['schemas']['TenantSummary'][]
        }
      }
      401: components['responses']['Unauthorized']
    }
  }
}
