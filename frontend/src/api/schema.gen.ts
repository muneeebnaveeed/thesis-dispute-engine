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
    get?: never
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
      | 'malformed-request'
      | 'contract-violation'
      | 'not-found'
      | 'invalid-transition'
      | 'appeals-exhausted'
      | 'concurrent-update'
      | 'idempotency-key-reuse'
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
    /** @enum {string} */
    Regime: 'EU_SEPA_DIRECT_DEBIT' | 'EU_PSD2_CARD' | 'US_REG_E' | 'US_REG_Z'
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
      /**
       * @description Who opened it; defaults to customer.
       * @default customer
       */
      actor?: string
    }
    ApplyEventRequest: {
      event: components['schemas']['DisputeEvent']
      /** @default system */
      actor?: string
      /** @description Event-specific facts, stored verbatim on the log entry. */
      payload?: {
        [key: string]: unknown
      }
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
    Dispute: {
      /** Format: uuid */
      id: string
      regime: components['schemas']['Regime']
      state: components['schemas']['DisputeState']
      appeals: number
      /** Format: int64 */
      version: number
      /** Format: uuid */
      transactionId: string
      /** Format: uuid */
      accountId: string
      /**
       * @description Decimal as a string; never a float.
       * @example 125.4000
       */
      disputedAmount: string
      currency: string
      /** Format: date-time */
      openedAt: string
      /** Format: date-time */
      updatedAt: string
      allowedEvents: components['schemas']['DisputeEvent'][]
      events: components['schemas']['LoggedEvent'][]
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
    /** @description Missing, unknown or revoked tenant key. */
    Unauthorized: {
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
    /** @description Well-formed but not acceptable (idempotency key reused with a different body, or no regime for the transaction). */
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
    }
  }
}
