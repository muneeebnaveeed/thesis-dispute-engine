import { trace } from '@opentelemetry/api'
import { SeverityNumber, type AnyValueMap } from '@opentelemetry/api-logs'

import { logger } from './sdk'

type Level = 'info' | 'warn' | 'error'
const SEVERITY: Record<Level, SeverityNumber> = {
  info: SeverityNumber.INFO,
  warn: SeverityNumber.WARN,
  error: SeverityNumber.ERROR,
}

// one line to stdout for docker logs, one record to Loki with the active trace attached; never a token or a body
const emit = (level: Level, message: string, attributes: AnyValueMap = {}) => {
  const span = trace.getActiveSpan()?.spanContext()
  const line = {
    level,
    msg: message,
    ...attributes,
    ...(span ? { trace_id: span.traceId, span_id: span.spanId } : {}),
  }
  console[level === 'info' ? 'log' : level](JSON.stringify(line))
  logger().emit({
    severityNumber: SEVERITY[level],
    severityText: level.toUpperCase(),
    body: message,
    attributes,
  })
}

export const log = {
  info: (message: string, attributes?: AnyValueMap) => emit('info', message, attributes),
  warn: (message: string, attributes?: AnyValueMap) => emit('warn', message, attributes),
  error: (message: string, attributes?: AnyValueMap) => emit('error', message, attributes),
}
