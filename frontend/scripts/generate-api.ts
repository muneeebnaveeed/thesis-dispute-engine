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

// $refs become references to sibling exports, so emission order must follow dependencies
const schemas = spec.components.schemas
type JsonSchemaNode = Record<string, unknown>
const typeBoxOptions = (node: JsonSchemaNode, keys: string[]) => {
  const kept: JsonSchemaNode = {}
  for (const key of keys) if (node[key] !== undefined) kept[key] = node[key]
  return Object.keys(kept).length ? `, ${JSON.stringify(kept)}` : ''
}
const emitTypeBox = (node: JsonSchemaNode): string => {
  if (typeof node.$ref === 'string') return node.$ref.replace('#/components/schemas/', '')
  if (Array.isArray(node.allOf)) {
    return `Type.Intersect([${(node.allOf as JsonSchemaNode[]).map(emitTypeBox).join(', ')}])`
  }
  const options = typeBoxOptions(node, [
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
  if (Array.isArray(node.enum)) {
    const literals = (node.enum as string[])
      .map((value) => `Type.Literal(${JSON.stringify(value)})`)
      .join(', ')
    return `Type.Union([${literals}]${options})`
  }
  switch (node.type) {
    case 'string':
      return `Type.String(${options.slice(2)})`
    case 'integer':
      return `Type.Integer(${options.slice(2)})`
    case 'number':
      return `Type.Number(${options.slice(2)})`
    case 'boolean':
      return `Type.Boolean(${options.slice(2)})`
    case 'array':
      return `Type.Array(${emitTypeBox(node.items as JsonSchemaNode)}${options})`
    case 'object': {
      const properties = (node.properties ?? {}) as Record<string, JsonSchemaNode>
      const required = new Set((node.required ?? []) as string[])
      if (Object.keys(properties).length === 0) {
        const additional = node.additionalProperties
        const valueSchema =
          additional && typeof additional === 'object'
            ? emitTypeBox(additional as JsonSchemaNode)
            : 'Type.Unknown()'
        return `Type.Record(Type.String(), ${valueSchema}${options})`
      }
      const propertyLines = Object.entries(properties)
        .map(
          ([name, schema]) =>
            `  ${name}: ${required.has(name) ? emitTypeBox(schema) : `Type.Optional(${emitTypeBox(schema)})`},`,
        )
        .join('\n')
      const closed = node.additionalProperties === false ? ', { additionalProperties: false }' : ''
      return `Type.Object({\n${propertyLines}\n}${closed || options})`
    }
    default:
      throw new Error(`unsupported schema: ${JSON.stringify(node)}`)
  }
}

const emissionOrder: string[] = []
const emitted = new Set<string>()
const collectRefs = (node: unknown, refs: Set<string>) => {
  if (Array.isArray(node)) node.forEach((item) => collectRefs(item, refs))
  else if (node && typeof node === 'object') {
    const schemaNode = node as JsonSchemaNode
    if (typeof schemaNode.$ref === 'string') refs.add(schemaNode.$ref.replace('#/components/schemas/', ''))
    Object.values(schemaNode).forEach((child) => collectRefs(child, refs))
  }
}
const emitAfterDependencies = (name: string) => {
  if (emitted.has(name)) return
  emitted.add(name)
  const dependencies = new Set<string>()
  collectRefs(schemas[name], dependencies)
  dependencies.forEach(emitAfterDependencies)
  emissionOrder.push(name)
}
Object.keys(schemas).toSorted().forEach(emitAfterDependencies)

const lines = [
  header,
  "import { Type, type Static } from '@sinclair/typebox'",
  "import type { components } from './schema.gen'",
  '',
  '// Each schema is checked both ways against the openapi-typescript type, so the two generated files cannot drift.',
  'type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : never) : never',
  '',
]
for (const name of emissionOrder) {
  lines.push(`export const ${name} = ${emitTypeBox(schemas[name] as JsonSchemaNode)}`)
  lines.push(`export type ${name} = Static<typeof ${name}>`)
  lines.push(`const _${name}: Same<${name}, components['schemas']['${name}']> = true`)
  lines.push(`void _${name}`, '')
}
await writeFile(path.join(outDir, 'schemas.gen.ts'), lines.join('\n'))
console.log(`generated ${emissionOrder.length} schemas into src/api`)
