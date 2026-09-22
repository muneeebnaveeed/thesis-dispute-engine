// The CLI does not understand our `#/` subpath alias for the class helper: it emits a bare `cn` specifier and
// installs an unrelated package of that name. It also writes sibling imports with a `.tsx` extension. Run it
// through this wrapper so every `add` lands normalised: pnpm shadcn add button
import { spawnSync } from 'node:child_process'
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const VENDORED = 'src/components/shadcn'

const run = (command: string, args: string[]) => spawnSync(command, args, { stdio: 'inherit' })

const cli = run('pnpm', ['dlx', 'shadcn@latest', ...process.argv.slice(2)])
if (cli.status !== 0) process.exit(cli.status ?? 1)

for (const file of readdirSync(VENDORED).filter((name) => name.endsWith('.tsx'))) {
  const path = join(VENDORED, file)
  const source = readFileSync(path, 'utf8')
  const normalised = source
    .replaceAll('from "cn"', 'from "#/lib/utils"')
    .replaceAll(/from "(#\/components\/shadcn\/[a-z-]+)\.tsx"/g, 'from "$1"')
  if (normalised !== source) {
    writeFileSync(path, normalised)
    console.log(`normalised ${path}`)
  }
}

const manifest: { dependencies?: Record<string, string> } = JSON.parse(readFileSync('package.json', 'utf8'))
if (manifest.dependencies?.cn) run('pnpm', ['remove', 'cn'])
run('pnpm', ['exec', 'oxfmt', VENDORED])
