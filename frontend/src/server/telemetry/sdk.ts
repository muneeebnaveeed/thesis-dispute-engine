import { metrics, trace, type Meter, type Tracer } from '@opentelemetry/api'
import { logs, type Logger } from '@opentelemetry/api-logs'
import { UndiciInstrumentation } from '@opentelemetry/instrumentation-undici'
import { resourceFromAttributes } from '@opentelemetry/resources'
import { NodeSDK } from '@opentelemetry/sdk-node'
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION, ATTR_URL_FULL } from '@opentelemetry/semantic-conventions'

const SCOPE = 'dispute-workbench'

let started: NodeSDK | null = null

const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi

// a session id in the path of an internal call is the browser's credential; it never reaches a trace
const maskedUrl = (request: { origin: string; path: string }) => ({
  [ATTR_URL_FULL]: `${request.origin}${request.path.replace(UUID, ':id')}`,
})

// exporters, sampler and propagators come from the same OTEL_* variables the API reads (deploy/otel.env); with no
// endpoint the API is noop and the process exports nothing, like the Go service. Undici instrumentation hooks
// diagnostics_channel, so fetch to the API carries traceparent without a module loader.
export const startTelemetry = (): void => {
  if (started || !process.env.OTEL_EXPORTER_OTLP_ENDPOINT) return
  started = new NodeSDK({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME ?? SCOPE,
      [ATTR_SERVICE_VERSION]: process.env.APP_VERSION ?? 'dev',
    }),
    instrumentations: [new UndiciInstrumentation({ requireParentforSpans: true, startSpanHook: maskedUrl })],
  })
  started.start()
  const stop = () => void started?.shutdown().catch(() => undefined)
  process.once('SIGTERM', stop)
  process.once('SIGINT', stop)
}

export const tracer = (): Tracer => trace.getTracer(SCOPE)
export const meter = (): Meter => metrics.getMeter(SCOPE)
export const logger = (): Logger => logs.getLogger(SCOPE)
