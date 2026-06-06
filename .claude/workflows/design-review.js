export const meta = {
  name: 'design-review',
  description: 'Review the current diff across design dimensions, adversarially verify each finding',
  whenToUse: 'After making UI changes — reviews the diff for token usage, spec-conformance, a11y, and visual-regression risk, keeping only findings that survive adversarial verification. Complements /code-review (which targets correctness).',
  phases: [
    { title: 'Review', detail: 'one agent per design dimension over the diff' },
    { title: 'Verify', detail: 'adversarially confirm each finding is real' },
  ],
}

// args (optional):
//   args.base : string   git ref to diff against (default: 'main')
const BASE = (args && args.base) || 'main'

const DIMENSIONS = [
  { key: 'tokens', prompt: 'Token usage: hardcoded hex/rgb/oklch instead of semantic tokens; tokens used outside their semantic role; radii ignoring the --radius scale.' },
  { key: 'spec', prompt: 'Spec conformance vs design/ui-design-spec.md: Material-isms, card treatment where flat document surfaces are wanted, forbidden selection markers, off-spec contrast.' },
  { key: 'a11y', prompt: 'Accessibility: missing focus-visible states, insufficient contrast, non-semantic interactive elements, missing aria/labels.' },
  { key: 'visual-regression', prompt: 'Visual-regression risk: layout/spacing/overflow changes, dark-mode (.dark) parity, responsive breakage.' },
]

const FINDINGS_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['dimension', 'findings'],
  properties: {
    dimension: { type: 'string' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['file', 'title', 'detail', 'severity'],
        properties: {
          file: { type: 'string' },
          title: { type: 'string' },
          detail: { type: 'string', description: 'the issue with the offending diff snippet' },
          severity: { type: 'string', enum: ['high', 'medium', 'low'] },
        },
      },
    },
  },
}

const VERDICT_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['real', 'reason'],
  properties: {
    real: { type: 'boolean', description: 'true if the finding is a genuine issue in the diff' },
    reason: { type: 'string' },
  },
}

const diffCmd = `git diff ${BASE}...HEAD -- 'aladin_react/**' && echo '--- name-status ---' && git diff --name-status ${BASE}...HEAD -- 'aladin_react/**'`

log(`Reviewing diff vs ${BASE} across ${DIMENSIONS.length} design dimensions.`)

const results = await pipeline(
  DIMENSIONS,
  // Stage 1 — review the diff through one dimension's lens.
  (dim) => agent(
    `Review the current frontend diff for the Aladin design system, through ONE lens.

Get the diff by running:
    ${diffCmd}

LENS — ${dim.key}: ${dim.prompt}

References: design/ui-design-spec.md, design/OVERHAUL.md (token bridge),
aladin_react/src/index.css (real tokens), CLAUDE.md (conventions).

Report ONLY issues introduced/changed by this diff, within this lens. Be conservative —
no nitpicks, no pre-existing issues. Set dimension="${dim.key}".`,
    { label: `review:${dim.key}`, phase: 'Review', schema: FINDINGS_SCHEMA, agentType: 'Explore' },
  ),
  // Stage 2 — adversarially verify each finding from this dimension.
  (review) => parallel((review && review.findings || []).map((f) => () =>
    agent(
      `Adversarially verify this design-review finding against the actual diff (vs ${BASE}).

FINDING: [${f.severity}] ${f.title}
FILE: ${f.file}
DETAIL: ${f.detail}

Inspect the real code/diff for ${f.file}. Try to REFUTE it. Default real=false if the
finding is a nitpick, pre-existing (not introduced by this diff), or not actually present.
Only real=true if it is a genuine issue introduced by this diff.`,
      { label: `verify:${f.title.slice(0, 32)}`, phase: 'Verify', schema: VERDICT_SCHEMA, agentType: 'Explore' },
    ).then((v) => ({ ...f, dimension: review.dimension, verdict: v })),
  )),
)

const confirmed = results
  .flat()
  .filter(Boolean)
  .filter((f) => f.verdict && f.verdict.real)
  .sort((a, b) => ({ high: 0, medium: 1, low: 2 }[a.severity] - { high: 0, medium: 1, low: 2 }[b.severity]))

log(`Confirmed ${confirmed.length} design finding(s) after adversarial verification.`)
return { base: BASE, confirmed }
