export const meta = {
  name: 'design-audit',
  description: 'Audit UI components against the Aladin design spec + live tokens, then record findings',
  whenToUse: 'When you want a systematic pass over UI surfaces for token drift, hardcoded colors, Material-isms, and spec violations.',
  phases: [
    { title: 'Audit', detail: 'one agent per component file, checked against the spec + token bridge' },
    { title: 'Record', detail: 'append the findings to design/OVERHAUL.md' },
  ],
}

// args (all optional):
//   args.targets : string[]  explicit file globs/paths to audit
//                            (default: components/ui + modules/*/ui)
//   args.specPath: string    design intent doc (default design/ui-design-spec.md)
const SPEC = (args && args.specPath) || 'design/ui-design-spec.md'
const TARGETS = (args && args.targets && args.targets.length)
  ? args.targets
  : [
      'aladin_react/src/components/ui/button.tsx',
      'aladin_react/src/components/ui/badge.tsx',
      'aladin_react/src/components/ui/card.tsx',
      'aladin_react/src/components/ui/dialog.tsx',
      'aladin_react/src/components/ui/dropdown-menu.tsx',
      'aladin_react/src/components/ui/tabs.tsx',
      'aladin_react/src/components/ui/input.tsx',
      'aladin_react/src/components/ui/textarea.tsx',
      'aladin_react/src/components/ui/scroll-area.tsx',
      'aladin_react/src/components/ui/aladin.tsx',
      'aladin_react/src/modules/workspace/ui/workspace-shell-ui.tsx',
      'aladin_react/src/modules/workspace/ui/browser-pane-ui.tsx',
      'aladin_react/src/modules/workspace/ui/work-pane-ui.tsx',
      'aladin_react/src/modules/sources/ui/integrations-dialog-ui.tsx',
      'aladin_react/src/modules/sources/ui/sources-overview-ui.tsx',
      'aladin_react/src/modules/pages/ui/page-editor-ui.tsx',
      'aladin_react/src/modules/artifacts/ui/artifact-ui.tsx',
      'aladin_react/src/modules/auth/ui/auth-ui.tsx',
    ]

const FINDINGS_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['file', 'conforms', 'findings'],
  properties: {
    file: { type: 'string' },
    conforms: { type: 'boolean', description: 'true if no issues found' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['severity', 'category', 'detail', 'suggestion'],
        properties: {
          severity: { type: 'string', enum: ['high', 'medium', 'low'] },
          category: {
            type: 'string',
            enum: ['hardcoded-color', 'token-misuse', 'material-ism', 'radius-drift', 'spec-violation', 'a11y', 'other'],
          },
          detail: { type: 'string', description: 'what is wrong, with the offending snippet/line if possible' },
          suggestion: { type: 'string', description: 'the concrete fix (e.g. which semantic token to use)' },
        },
      },
    },
  },
}

phase('Audit')
const results = await parallel(TARGETS.map((file) => () =>
  agent(
    `You are auditing one React/Tailwind component for the Aladin design system.

FILE TO AUDIT: ${file}

Read it, then read these references:
  - ${SPEC}                       (design intent / principles; AladinColor vocabulary)
  - design/OVERHAUL.md            (token bridge: spec role -> live shadcn/OKLch token)
  - aladin_react/src/index.css    (the actual tokens that exist)

Report violations of the design system. Specifically look for:
  - hardcoded-color: literal hex/rgb/oklch/hsl values instead of semantic tokens
  - token-misuse: using a token whose semantic role doesn't match its use
  - material-ism: Material-style ripple/filled/elevation/softness the spec rejects
  - radius-drift: radii that ignore the --radius scale (spec wants small 4-6px radii)
  - spec-violation: anything contradicting ${SPEC} (e.g. cards used where the spec
    wants flat document surfaces; selection markers the spec forbids)
  - a11y: missing focus-visible, insufficient contrast, non-semantic interactive els

Be precise and conservative: only report real issues, quote the offending code, and
give the concrete fix (which semantic token / Tailwind class to use). If the file is
clean, set conforms=true and findings=[]. Set the "file" field to exactly: ${file}`,
    { label: `audit:${file.split('/').pop()}`, phase: 'Audit', schema: FINDINGS_SCHEMA, agentType: 'Explore' },
  ),
))
const audited = results.filter(Boolean)
const withIssues = audited.filter((r) => r && !r.conforms && r.findings && r.findings.length)
log(`Audited ${audited.length} files; ${withIssues.length} have findings.`)

phase('Record')
if (!withIssues.length) {
  log('No findings — nothing to record.')
  return { audited: audited.length, findings: 0 }
}

await agent(
  `Append the audit findings below to design/OVERHAUL.md.

Edit the file: under the "### Findings (filled by audit workflow)" heading in section 3,
replace the "_(none yet ...)_" placeholder (or append, if findings already exist) with a
dated-by-run bullet list grouped by file. For each finding use the format:
  - [SEVERITY][category] file: detail — fix: suggestion
Also flip the matching backlog checkbox from ☐ to ◐ for any file that has findings.
Do not change anything else in the doc.

FINDINGS (JSON):
${JSON.stringify(withIssues, null, 2)}`,
  { label: 'record-findings', phase: 'Record' },
)

return { audited: audited.length, filesWithFindings: withIssues.length, findings: withIssues }
