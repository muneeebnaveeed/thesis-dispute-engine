import { Type } from '@sinclair/typebox'

// Forms that are not contract requests still validate with a schema, so every input goes through one path.
export const FindTenantInput = Type.Object({
  email: Type.String({ format: 'email', maxLength: 254 }),
})
