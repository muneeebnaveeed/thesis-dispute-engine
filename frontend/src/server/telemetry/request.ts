import { SpanKind, SpanStatusCode, context, propagation, trace } from '@opentelemetry/api'
import {
  ATTR_HTTP_REQUEST_METHOD,
  ATTR_HTTP_RESPONSE_STATUS_CODE,
  ATTR_HTTP_ROUTE,
  ATTR_URL_PATH,
} from '@opentelemetry/semantic-conventions'

import { meter, tracer } from './sdk'

const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi

// a span name must be bounded: ids become placeholders, an RPC is named after the function it calls
export const routeOf = (url: URL): string => {
  if (url.pathname.startsWith('/_serverFn/')) return '/_serverFn/:fn'
  return url.pathname.replace(UUID, ':id').replace(/\/\d+(?=\/|$)/g, '/:n')
}

const requestDuration = () =>
  meter().createHistogram('http.server.request.duration', {
    unit: 's',
    description: 'Time to answer an HTTP request at the workbench server',
  })

// one server span per HTTP request, continuing an incoming traceparent (a browser never sends one, a probe might)
export const traceRequest = async (request: Request, next: () => Promise<Response>): Promise<Response> => {
  const url = new URL(request.url)
  const route = routeOf(url)
  const parent = propagation.extract(context.active(), Object.fromEntries(request.headers))
  const span = tracer().startSpan(
    `${request.method} ${route}`,
    {
      kind: SpanKind.SERVER,
      attributes: {
        [ATTR_HTTP_REQUEST_METHOD]: request.method,
        [ATTR_HTTP_ROUTE]: route,
        [ATTR_URL_PATH]: url.pathname,
      },
    },
    parent,
  )
  const startedAt = performance.now()
  try {
    const response = await context.with(trace.setSpan(parent, span), next)
    span.setAttribute(ATTR_HTTP_RESPONSE_STATUS_CODE, response.status)
    if (response.status >= 500) span.setStatus({ code: SpanStatusCode.ERROR })
    requestDuration().record((performance.now() - startedAt) / 1000, {
      [ATTR_HTTP_REQUEST_METHOD]: request.method,
      [ATTR_HTTP_ROUTE]: route,
      [ATTR_HTTP_RESPONSE_STATUS_CODE]: response.status,
    })
    return response
  } catch (thrown) {
    span.setStatus({
      code: SpanStatusCode.ERROR,
      message: thrown instanceof Error ? thrown.message : String(thrown),
    })
    if (thrown instanceof Error) span.recordException(thrown)
    throw thrown
  } finally {
    span.end()
  }
}
