# Linear-Meet-Notion Marketing System Reference

Use this reference when you need concrete implementation detail. The goal is not to clone either source brand, but to reproduce a specific blend for a knowledge base platform:

- Linear's precision, contrast discipline, and product sharpness
- Notion's editorial calm, readable whitespace, and soft utility

Default subject matter:

- search
- documents
- notes
- linked knowledge
- research flows
- knowledge retrieval
- team memory

## Design intent

The surface should read as:

- intelligent but not arrogant
- precise but not cold
- editorial but not luxurious
- minimal but not empty
- premium but not showy
- calm enough to trust with serious information

The page should feel assembled from a few strong decisions rather than many decorative ones.

## Suggested tokens

### Color

```yaml
colors:
  primary: "#111111"
  primary-active: "#242424"
  canvas: "#fcfcfb"
  surface-soft: "#f7f6f3"
  surface-card: "#f3f2ef"
  surface-strong: "#e7e5e0"
  surface-dark: "#101010"
  surface-dark-elevated: "#1a1a1a"
  hairline: "#e7e5e0"
  hairline-soft: "#f1efeb"
  ink: "#111111"
  body: "#3f3f46"
  muted: "#6b7280"
  muted-soft: "#8b8b92"
  on-primary: "#ffffff"
  on-dark: "#ffffff"
  on-dark-soft: "#a1a1aa"
  success: "#10b981"
  warning: "#f59e0b"
  error: "#ef4444"
```

### Typography

```yaml
typography:
  display-xl: { size: 64, weight: 600, lineHeight: 1.02, tracking: "-0.05em" }
  display-lg: { size: 48, weight: 600, lineHeight: 1.08, tracking: "-0.04em" }
  display-md: { size: 36, weight: 600, lineHeight: 1.12, tracking: "-0.025em" }
  display-sm: { size: 28, weight: 600, lineHeight: 1.18, tracking: "-0.015em" }
  title-lg: { size: 22, weight: 600, lineHeight: 1.3 }
  title-md: { size: 18, weight: 600, lineHeight: 1.4 }
  title-sm: { size: 16, weight: 600, lineHeight: 1.4 }
  body-md: { size: 16, weight: 400, lineHeight: 1.5 }
  body-sm: { size: 14, weight: 400, lineHeight: 1.55 }
  caption: { size: 13, weight: 500, lineHeight: 1.35 }
  button: { size: 14, weight: 600, lineHeight: 1.0 }
  nav-link: { size: 14, weight: 500, lineHeight: 1.4 }
```

Recommended font pairing:

- Display: precise neo-grotesk or geometric sans with restrained character
- Body/UI: neutral sans with strong editorial readability

Good fallback combinations:

- Display: Inter, Geist, Suisse Int'l-style, or Manrope with tuned tracking
- Body/UI: Inter, IBM Plex Sans, or system UI sans

### Spacing and shape

```yaml
spacing:
  xxs: 4
  xs: 8
  sm: 12
  md: 16
  lg: 24
  xl: 32
  xxl: 48
  section: 104

radius:
  xs: 4
  sm: 6
  md: 8
  lg: 10
  xl: 14
  pill: 9999
  full: 9999
```

## Layout rules

- Max content width: about `1200px`
- Hero split on desktop: roughly `7/5`
- Feature grid: `3 -> 2 -> 1`
- Pricing grid: `4 -> 2 -> 1`
- Footer: `4 -> 2 -> 1`
- Section padding: about `104px` vertical

Layout tone:

- Linear side: tighter grouping, cleaner alignment, stronger contrast around search, structure, and proof sections
- Notion side: more paragraph air, quieter surfaces, more relaxed reading rhythm in narrative bands

Whitespace should support fast scanning. Each band should have one headline, one supporting paragraph, and one dominant action.

## Surface rhythm

Use this cadence:

1. White hero
2. Soft-neutral feature or proof cards
3. White product-led section
4. Optional soft CTA card
5. Dark footer

Do not stack multiple unrelated dark surfaces above the footer. Dark should be the exception, not the baseline.

## Component patterns

### Top nav

- Warm-white bar
- Wordmark left
- Simple nav links center or right
- Text link plus one strong CTA on the right
- Collapse to hamburger below `768px`
- Navigation labels should feel informational and grounded, not salesy

### Primary button

- Near-black fill
- White text
- `40px` height
- `8px` radius
- Semibold label

### Secondary button

- White fill
- Hairline border
- Near-black text

### Pill switcher

- Soft neutral outer pill
- White active segment
- Small internal shadow on active segment only

### Feature card

- Soft neutral background
- `10px` radius
- `32px` padding
- Small icon
- Tight, readable title and body
- Favor quiet information density over oversized marketing copy
- Good subjects: search speed, source coverage, linked notes, summaries, collections, permissions, retrieval, citation trails

### Product mockup card

- White background
- Thin border or very soft shadow
- Real UI fragment inside
- Preserve product chrome; do not overdecorate the container
- Product shots should feel exact and crisp, almost like documentation elevated into marketing
- Prefer UI fragments such as:
  - search input and ranked results
  - document reader with highlights
  - graph or relation map
  - saved collection or notebook view
  - filters, sources, and metadata panels

### Quote card

- Soft neutral background
- `24px` padding
- Circular `36px` avatar
- Short quote, not long wall-of-text testimonials
- Keep the tone literate and credible, not gushy
- Use sparingly. Knowledge products earn trust more through product clarity than through heavy social-proof density.

### Pricing

- Standard tiers on white
- Optional featured tier on dark background with inverted text
- Let contrast do the emphasis rather than large scaling or bright badges
- Plan layout should feel structured enough to scan like a product table

### Footer

- Near-black background
- Muted light text
- Link columns
- This is the page closer; keep it visually grounded and simple

## Responsive rules

### Mobile

- Collapse hero to one column
- Put product mockup after copy
- Turn grids into single-column stacks
- Reflow, do not shrink UI fragments to illegibility
- Preserve generous margins so the page still feels editorial, not cramped

### Tablet

- Allow pill groups and nav clusters to wrap
- Reduce large grids to two columns

### Desktop

- Preserve generous whitespace
- Keep line lengths controlled even when the viewport is wide

## Content guidance

- Write headlines with crisp confidence, not hype.
- Keep subcopy functional, literate, and specific.
- Show product truth whenever possible: search results, docs surfaces, note views, knowledge graphs, collections, sources, citations, filters, and editor states.
- Treat illustrations as fallback, not default.

Voice calibration:

- More Linear: "Find what matters before the tab chaos starts."
- More Notion: "Your team's knowledge, kept clear and close."

The blend should sound clear and adult, not witty for its own sake.

## Anti-patterns

- Bright gradient hero backgrounds
- Colored primary buttons
- Colored badges or decorative accents on the page shell
- Multiple featured cards competing at once
- Excessive corner radii beyond `14px` on main cards
- Heavy shadow stacks
- Placeholder dashboard art that does not resemble the actual product
- Cute productivity mascot energy
- Hyper-futurist chrome or cyber gloss
