// Generates src/api/*.gen.ts from docs/api/openapi.yaml: TypeScript types via openapi-typescript and one TypeBox
// schema per component, paired with its generated type through Type.Unsafe so form validation and the client agree.
import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import openapiTS, { astToString } from 'openapi-typescript'
import { parse } from 'yaml'

const here = path.dirname(fileURLToPath(import.meta.url))
const specPath = path.resolve(here, '../../docs/api/openapi.yaml')
const outDir = path.resolve(here, '../src/api')
const header = '// Generated from docs/api/openapi.yaml by scripts/generate-api.ts. Do not edit.\n'

const specText = await readFile(specPath, 'utf8')
const spec = parse(specText) as { components: { schemas: Record<string, unknown> } }

const types = astToString(
  await openapiTS(new URL(`file://${specPath}`), {
    exportType: true,
    rootTypes: false,
    defaultNonNullable: false,
  }),
)
await writeFile(path.join(outDir, 'schema.gen.ts'), header + types)

// Emit TypeBox builder calls for the JSON Schema subset the spec uses; $refs point at sibling exports.
const schemas = spec.components.schemas
type Node = Record<string, unknown>
const opt = (n: Node, keys: string[]) => {
  const o: Node = {}
  for (const k of keys) if (n[k] !== undefined) o[k] = n[k]
  return Object.keys(o).length ? `, ${JSON.stringify(o)}` : ''
}
function emit(n: Node): string {
  if (typeof n.$ref === 'string') return n.$ref.replace('#/components/schemas/', '')
  const meta = opt(n, [
    'description',
    'default',
    'format',
    'minimum',
    'maximum',
    'minLength',
    'maxLength',
    'pattern',
    'example',
  ])
  if (Array.isArray(n.enum)) {
    const lits = (n.enum as string[]).map((v) => `Type.Literal(${JSON.stringify(v)})`).join(', ')
    return `Type.Union([${lits}]${meta})`
  }
  switch (n.type) {
    case 'string':
      return `Type.String(${meta.slice(2)})`
    case 'integer':
      return `Type.Integer(${meta.slice(2)})`
    case 'number':
      return `Type.Number(${meta.slice(2)})`
    case 'boolean':
      return `Type.Boolean(${meta.slice(2)})`
    case 'array':
      return `Type.Array(${emit(n.items as Node)}${meta})`
    case 'object': {
      const props = (n.properties ?? {}) as Record<string, Node>
      const required = new Set((n.required ?? []) as string[])
      if (Object.keys(props).length === 0) {
        const ap = n.additionalProperties
        const value = ap && typeof ap === 'object' ? emit(ap as Node) : 'Type.Unknown()'
        return `Type.Record(Type.String(), ${value}${meta})`
      }
      const body = Object.entries(props)
        .map(([k, v]) => `  ${k}: ${required.has(k) ? emit(v) : `Type.Optional(${emit(v)})`},`)
        .join('\n')
      const ap = n.additionalProperties === false ? ', { additionalProperties: false }' : ''
      return `Type.Object({\n${body}\n}${ap || meta})`
    }
    default:
      throw new Error(`unsupported schema: ${JSON.stringify(n)}`)
  }
}

// Emit in dependency order so a schema's $ref targets are declared first.
const order: string[] = []
const seen = new Set<string>()
const refsOf = (n: unknown, acc: Set<string>) => {
  if (Array.isArray(n)) n.forEach((x) => refsOf(x, acc))
  else if (n && typeof n === 'object') {
    const o = n as Node
    if (typeof o.$ref === 'string') acc.add(o.$ref.replace('#/components/schemas/', ''))
    Object.values(o).forEach((v) => refsOf(v, acc))
  }
}
const visit = (name: string) => {
  if (seen.has(name)) return
  seen.add(name)
  const deps = new Set<string>()
  refsOf(schemas[name], deps)
  deps.forEach(visit)
  order.push(name)
}
Object.keys(schemas).toSorted().forEach(visit)

const lines = [
  header,
  "import { Type, type Static } from '@sinclair/typebox'",
  "import type { components } from './schema.gen'",
  '',
  '// Each schema is checked both ways against the openapi-typescript type, so the two generated files cannot drift.',
  'type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : never) : never',
  '',
]
for (const name of order) {
  lines.push(`export const ${name} = ${emit(schemas[name] as Node)}`)
  lines.push(`export type ${name} = Static<typeof ${name}>`)
  lines.push(`const _${name}: Same<${name}, components['schemas']['${name}']> = true`)
  lines.push(`void _${name}`, '')
}
await writeFile(path.join(outDir, 'schemas.gen.ts'), lines.join('\n'))
console.log(`generated ${order.length} schemas into src/api`)
