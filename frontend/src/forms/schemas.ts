import { Type } from '@sinclair/typebox'

export const FindTenantInput = Type.Object({
  email: Type.String({ format: 'email', maxLength: 254 }),
})
