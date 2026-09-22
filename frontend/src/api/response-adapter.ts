import { createSerializationAdapter } from '@tanstack/react-router'

// lets a server function return an openapi-fetch result as it is: the Response crosses the RPC boundary as its
// status line only (the body was consumed on the server), so no wrapper has to strip it
export const responseAdapter = createSerializationAdapter({
  key: 'http-response',
  test: (value): value is Response => value instanceof Response,
  toSerializable: (response) => ({ status: response.status, statusText: response.statusText }),
  fromSerializable: ({ status, statusText }) => new Response(null, { status, statusText }),
})
