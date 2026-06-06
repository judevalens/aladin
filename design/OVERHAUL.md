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
- ☐ `button.tsx` — CVA reference component; confirm variant set matches spec controls
- ☐ `badge.tsx` — chips/labels; spec says tags shouldn't look like buttons unless interactive
- ☐ `card.tsx` — spec: panels are document surfaces, "not cards"; reconcile usage
- ☐ `dialog.tsx` — management modals may use stronger black/white contrast
- ☐ `dropdown-menu.tsx`
- ☐ `tabs.tsx`
- ☐ `input.tsx`
- ☐ `textarea.tsx`
- ☐ `scroll-area.tsx`
- ☐ `aladin.tsx` — custom shell panes; central to the three-pane spec

### Feature UI — `aladin_react/src/modules/*/ui/`
- ☐ `workspace/workspace-shell-ui.tsx` — the three-pane shell (app rail / browser / work pane)
- ☐ `workspace/browser-pane-ui.tsx` — dense browser rows, selection rules
- ☐ `workspace/work-pane-ui.tsx` — artifact/work pane
- ☐ `workspace/rename-dialog-ui.tsx`, `voice-capture-dialog-ui.tsx`
- ☐ `sources/integrations-dialog-ui.tsx`, `add-source-dialog-ui.tsx`,
     `source-details-dialog-ui.tsx`, `sources-overview-ui.tsx`, `sources-parts-ui.tsx`
     — wide management modals (the "focused app screen" treatment)
- ☐ `pages/page-editor-ui.tsx`, `page-history-panel.tsx`
- ☐ `artifacts/artifact-ui.tsx`
- ☐ `auth/auth-ui.tsx`

### Findings (filled by audit workflow)
_(none yet — run `.claude/workflows/design-audit.js`)_
