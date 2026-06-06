# Aladin Design-System Overhaul — North Star

Living doc for the upcoming UI/design-system overhaul. It exists to (1) reconcile the
two vocabularies the project currently speaks, (2) hold the candidate directions without
locking one in prematurely, and (3) track the per-surface audit backlog.

- **Design intent / principles:** `design/ui-design-spec.md` (the editorial,
  terminal-precise direction; written in the `AladinColor` vocabulary).
- **Live tokens:** `aladin_react/src/index.css` (Tailwind v4 + shadcn, OKLch).
- **Status:** direction **NOT yet locked** (see §2). Until locked, treat this doc as
  exploratory — don't mass-rename tokens or rewrite components.

---

## 1. Token bridge (`AladinColor` spec vocabulary ↔ live shadcn/OKLch tokens)

The spec describes roles; the code ships shadcn semantic tokens. This table is the
Rosetta stone — use the **live token** in code, reach for the spec term when reasoning
about intent. (Mappings marked *approx* need a design decision during the overhaul.)

| Spec role (`AladinColor`) | Live token (`index.css`) | Tailwind class | Notes |
|---|---|---|---|
| `Canvas` | `--background` | `bg-background` | white canvas / app base |
| `Panel` | `--card` | `bg-card` | document/workspace surfaces |
| `PanelMuted` | `--muted` | `bg-muted` | pane-2 / nav off-white *(approx)* |
| `RowHover` / `ControlHover` | `--accent` | `hover:bg-accent` | grayscale hover fills *(approx)* |
| `RowSelected` | `--accent` | `bg-accent` | light selected fill *(approx)* |
| `RowSelectedStrong` | `--secondary` | `bg-secondary` | stronger nav selection *(approx)* |
| `ControlPressed` | `--accent` (darker step) | — | needs a dedicated pressed token *(gap)* |
| `Divider` / `Border` | `--border` | `border-border` | thin structural dividers |
| `Ink` | `--foreground` | `text-foreground` | warm near-black primary text |
| `InkSecondary` | `--muted-foreground` | `text-muted-foreground` | secondary text |
| `InkMuted` / `InkDisabled` | `--muted-foreground` (+ opacity) | `text-muted-foreground/70` | *(approx)* |
| `InkSurface` | `--primary` | `bg-primary` | compact high-contrast controls / modal emphasis |
| `OnInkSurface` | `--primary-foreground` | `text-primary-foreground` | near-white on InkSurface |
| `InkSurfaceHover` | `--primary` (darker step) | — | needs a token *(gap)* |
| `ActiveMarker` | `--primary` / `--foreground` | — | row markers; *(approx, decide)* |
| `CommandSurface` | `--secondary` / `--muted` | — | command/search affordances *(approx)* |
| `CodeText` | `--muted-foreground` + mono | `font-mono text-muted-foreground` | metadata / technical labels |
| (chips/labels) | `--secondary` + `--border` | `badge` variants | see `components/ui/badge.tsx` |
| `--radius` family | `--radius` (0.625rem) | `rounded-{sm,md,lg,xl}` | spec wants small radius (4–6px) |

**Gaps to resolve in the overhaul:** dedicated *pressed* and *InkSurfaceHover* steps,
an explicit `ActiveMarker`, and a decision on whether the off-white ladder
(`Canvas`/`Panel`/`PanelMuted`) maps to `background`/`card`/`muted` or needs a new
`--surface-*` scale. Dark mode (`.dark` in `index.css`) must get the same treatment.

---

## 2. Candidate directions (pick one to lock)

Fill in the chosen one; leave the others as rejected-with-reason once decided.

### (a) Reconcile spec → tokens
Treat `ui-design-spec.md` as source of truth; port its vocabulary onto the live
shadcn/OKLch tokens, close the gaps above, then sweep components to use semantic tokens
only. Lowest risk, preserves current direction. *Status: candidate.*

### (b) Fresh visual redesign
Rethink the visual language itself. The harness supports exploring directions side by
side (branch per direction + visual diff via the Preview loop). Highest ceiling, most
work. *Status: candidate.*

### (c) Component-by-component polish
Keep the current direction; systematically audit and refine each surface in §3 against
the spec, fixing token drift and Material-isms incrementally. Lowest ceiling, ships
continuously. *Status: candidate.*

---

## 3. Component audit backlog

Seeded from `components/ui/*` and `modules/*/ui`. The `design-audit` workflow
(`.claude/workflows/design-audit.js`) appends concrete findings here. Status legend:
☐ not reviewed · ◐ in progress · ☑ conforms.

### Primitives — `aladin_react/src/components/ui/`
- ☑ `button.tsx` — token-clean REFERENCE component; the target style for the rest
- ◐ `badge.tsx` (9 hex) — default/muted/inverted variants hardcoded; map to tokens
- ◐ `card.tsx` (3 hex) — spec: panels are document surfaces, "not cards"; reconcile usage
- ◐ `dialog.tsx` (12 hex) — bespoke shadow elevations; management-modal contrast decision
- ◐ `dropdown-menu.tsx` (16 hex) — bespoke shadows + hex fills
- ◐ `tabs.tsx` (7 hex) — active-tab shadow + hex
- ◐ `input.tsx` (5 hex) — blue focus ring `#2563eb` (forbidden)
- ◐ `textarea.tsx` (5 hex) — blue focus ring `#2563eb` (forbidden)
- ☐ `scroll-area.tsx` — no hardcoded colors found; verify on closer read
- ◐ `aladin.tsx` (9 hex) — custom shell panes; central to the three-pane spec

### Feature UI — `aladin_react/src/modules/*/ui/`
- ◐ `workspace/workspace-shell-ui.tsx` (24 hex) — the three-pane shell
- ◐ `workspace/browser-pane-ui.tsx` (29 hex) — dense browser rows, selection rules
- ◐ `workspace/work-pane-ui.tsx` (24 hex) — artifact/work pane
- ◐ `workspace/voice-capture-dialog-ui.tsx` (4 hex); ☐ `rename-dialog-ui.tsx` (verify)
- ◐ `sources/integrations-dialog-ui.tsx` (32 — worst), `sources-overview-ui.tsx` (17),
     `sources-parts-ui.tsx` (11), `add-source-dialog-ui.tsx` (4),
     `source-details-dialog-ui.tsx` (1) — wide management modals
- ◐ `pages/page-history-panel.tsx` (21, `shadow-lg`); `pages/page-editor-ui.tsx` (3)
- ◐ `artifacts/artifact-ui.tsx` (11) — blue link accent `#2563eb`
- ◐ `auth/auth-ui.tsx` (20) — blue accent `#2563eb`
- ◐ `sources/sources-route.tsx` (2)

### Findings — Audit pass 1 (inline sweep)

**Headline: pervasive token bypass.** 172 hardcoded color values across the UI
(Tailwind arbitrary-value hex like `border-[#e7e5e4]`, `text-[#0a0a0a]`, plus 44×
`bg-white`). Only `button.tsx` is token-clean — it correctly uses `bg-primary`,
`text-foreground`, `border-border`. **Every other surface hand-codes resolved colors
and bypasses the token layer entirely.** This is the single biggest design-system debt.

**The de-facto palette → token mapping** (counts = occurrences):

| Hex | × | Role | Target token |
|---|---|---|---|
| `#e7e5e4` | 56 | divider/border | `--border` → `border-border` |
| `#0a0a0a` | 49 | primary ink | `--foreground` → `text-foreground` |
| `#78716c` | 28 | secondary/muted text | `--muted-foreground` |
| `#44403c` | 17 | body text | `text-foreground/80` *(decide)* |
| `#fafaf9` | 16 | canvas/panel-muted | `--background` / `--sidebar` |
| `#a8a29e` | 16 | placeholder/disabled | `--muted-foreground` (lighter) |
| `#f2f0ee` | 15 | hover/muted fill | `--muted` / `--accent` |
| `#57534e` | 15 | secondary text | `--muted-foreground` *(decide)* |
| `#d6d3d1` | 7 | border hover | needs a `--border-strong` *(gap)* |
| `#18181b` | 6 | ink surface | `--primary` (compact controls) |
| `bg-white` | 44 | surfaces | `--background` / `--card` |

**Critical mismatch — warm hex vs. neutral tokens.** The hand-coded palette is the
**warm `stone` family** (has chroma), matching the spec's "warm near-black ink, never
pure black." But the live OKLch tokens in `index.css` are **pure neutral gray**
(`oklch(L 0 0)`, chroma 0). So the components honor the spec's *intent* while the token
layer doesn't encode the warmth. **Reconciliation must decide:** warm up the tokens
(add chroma to match `stone`) and then sweep components to semantic tokens — vs. flatten
to neutral. The audit favors warming the tokens (keeps the spec's editorial feel).

**Spec violations — forbidden color (monochrome rule):**
- ☐ Blue accent `#2563eb` (6×): `input.tsx`, `textarea.tsx` (focus ring), `artifact-ui.tsx`
  (link), `auth-ui.tsx`. Spec forbids colorful accents → use `--ring` / `--foreground`.
- ☐ Purple `#7e22ce` / `#f3e8ff` (likely a tag): off-palette → monochrome or `--destructive`-style semantic only.
- ☐ Several reds (`#ef4444`/`#dc2626`/`#991b1b`/`#fef2f2`/`#fecaca`) → consolidate to `--destructive` family.

**Material-isms:** `shadow-lg` in `page-history-panel.tsx`; bespoke `shadow-[...]`
elevations in `dialog.tsx`, `dropdown-menu.tsx`, `tabs.tsx` — spec wants flat, structural
surfaces. Reconcile against the (gap) elevation decision.

**Worst offenders (hardcoded-hex count):** `integrations-dialog-ui.tsx` (32),
`browser-pane-ui.tsx` (29), `workspace-shell-ui.tsx` (24), `work-pane-ui.tsx` (24),
`page-history-panel.tsx` (21), `auth-ui.tsx` (20), `sources-overview-ui.tsx` (17).

**Recommended next step:** lock direction **(a) reconcile spec → tokens**: (1) decide
warm vs. neutral and encode it in `index.css` (incl. the `--border-strong` gap);
(2) run the `component-migration` workflow to mechanically replace hex → semantic tokens,
one file per agent, typechecked. `button.tsx` is the reference for the target style.
