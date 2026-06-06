export const meta = {
  name: 'component-migration',
  description: 'Apply one transformation across many component files, verifying each independently',
  whenToUse: 'When the same mechanical/design change must land across a known list of components (e.g. replace hardcoded colors with tokens, migrate a prop, adopt a new variant).',
  phases: [
    { title: 'Migrate', detail: 'one agent edits each target file (worktree-isolated)' },
    { title: 'Verify', detail: 'typecheck each migrated file and report status' },
  ],
}

// args (REQUIRED):
//   args.instruction : string    the transformation to apply to every target
//   args.targets     : string[]  file paths to migrate
// args (optional):
//   args.typecheckCmd: string    override the per-item verify command
const INSTRUCTION = args && args.instruction
const TARGETS = (args && args.targets) || []
const TYPECHECK = (args && args.typecheckCmd) ||
  'cd aladin_react && npx tsc --noEmit -p tsconfig.app.json'

if (!INSTRUCTION || !TARGETS.length) {
  log('component-migration requires args.instruction (string) and args.targets (string[]). Nothing to do.')
  return { migrated: 0, error: 'missing args.instruction or args.targets' }
}

const VERDICT_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['file', 'changed', 'typechecks', 'summary'],
  properties: {
    file: { type: 'string' },
    changed: { type: 'boolean', description: 'true if the file was edited' },
    typechecks: { type: 'boolean', description: 'true if the project typechecks after the edit' },
    summary: { type: 'string', description: 'one-line description of what was done / why it failed' },
  },
}

log(`Migrating ${TARGETS.length} file(s): ${INSTRUCTION}`)

const results = await pipeline(
  TARGETS,
  // Stage 1 — transform the file in an isolated worktree (parallel edits won't clash).
  (file) => agent(
    `Apply this transformation to a single file, then stop.

FILE: ${file}
TRANSFORMATION: ${INSTRUCTION}

Rules:
  - Edit ONLY ${file}. Do not touch other files.
  - Honor the Aladin conventions in CLAUDE.md and design/DESIGN_SPEC.md (semantic tokens
    via @/lib/utils cn(); no hardcoded colors; small radii; no Material-isms).
  - Keep the change minimal and behavior-preserving unless the instruction says otherwise.
  - If the transformation does not apply to this file, make no edit and say so.
Return your final text as a short summary of what you changed (or why you didn't).`,
    { label: `migrate:${file.split('/').pop()}`, phase: 'Migrate', isolation: 'worktree' },
  ).then((summary) => ({ file, summary })),

  // Stage 2 — verify the migrated file typechecks.
  (prev, file) => agent(
    `Run this command and report whether the project still typechecks after the edit to ${file}:

    ${TYPECHECK}

Report changed=true (an edit was made by the prior step: ${JSON.stringify(prev && prev.summary || '')}),
typechecks=(command exit 0), and a one-line summary. Set "file" to exactly: ${file}`,
    { label: `verify:${file.split('/').pop()}`, phase: 'Verify', schema: VERDICT_SCHEMA },
  ),
)

const verdicts = results.filter(Boolean)
const ok = verdicts.filter((v) => v.typechecks)
const broken = verdicts.filter((v) => !v.typechecks)
log(`Done: ${ok.length}/${verdicts.length} typecheck clean; ${broken.length} need attention.`)

return { migrated: verdicts.length, clean: ok.length, broken, verdicts }
