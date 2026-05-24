---
name: monochrome-saas-marketing-ui
description: "Design and implement calm monochrome product-marketing surfaces for a knowledge base platform, with a Linear-meets-Notion tone: precise hierarchy, editorial whitespace, crisp product framing, and restrained black-and-white contrast. Use this when creating or refining Aladin landing pages, feature bands, docs-adjacent storytelling sections, quote rows, CTA sections, or product-led homepages that should feel intelligent, trustworthy, and quietly premium without becoming flashy, playful, or clone-like."
---

# Linear-Meet-Notion Knowledge Product UI

Use this skill for marketing surfaces that should feel product-led, editorial, and quietly sharp. The target is not either source brand literally. It is the overlap:

- Linear: precision, crisp contrast, strong hierarchy, polished product framing
- Notion: calm whitespace, soft utility, editorial readability, low-drama surfaces

Default product context for this skill: a knowledge base platform such as Aladin. That means the interface should feel closer to thought organization, search, writing, connected knowledge, and retrieval than to sales-led SaaS or developer-infra marketing.

## Apply this skill when

- The user wants a landing page, homepage, feature page, docs-adjacent marketing section, or product storytelling surface.
- The target aesthetic is minimal, intelligent, calm, and product-forward.
- The page should show product UI fragments directly instead of relying on abstract illustrations.
- The product involves knowledge, search, notes, documents, connections, memory, research, or team understanding.

Do not use this skill when the repo already has a strong conflicting design system. In that case, preserve the established system first and borrow only compatible ideas.

## Core direction

Design toward these constraints:

- Calm white or warm-white page canvas with near-black primary actions.
- Tighter, more precise headline hierarchy than generic SaaS pages.
- Editorial body copy with clean rhythm and moderate line length.
- Soft neutral cards for feature claims, proof, and quote rows.
- Real product chrome embedded inside cards whenever possible.
- One primary CTA per band.
- Dark footer as the only persistent dark surface on the page.

Avoid ornamental gradients, heavy shadows, glass effects, colored accents, and startup-cute illustration systems.

## Workflow

1. Identify the page bands: hero, proof, feature explanation, product demo, workflow/story section, CTA, footer.
2. Assign one dominant purpose to each band. Do not mix multiple competing messages in the same section.
3. Set the tone balance per band:
   - more Linear for search, navigation, command flows, structured lists, and proof
   - more Notion for narrative copy, document surfaces, and editorial sections
4. Alternate surfaces to preserve rhythm: white band, soft-card cluster, white/product band, then dark footer.
5. Prefer product artifacts over invented illustrations. If the product exists, show search results, document views, graph relationships, saved notes, filters, sidebars, and connected-knowledge surfaces in cropped, legible fragments.
6. Keep actions sparse. Each band gets one primary action and at most one secondary action.
7. Validate that mobile stacking preserves hierarchy before finishing.

## Implementation rules

### Typography

- Use a clean, modern display face for headlines when available. It should feel precise, not expressive.
- If the brand display face is unavailable, approximate it with a semibold sans and slight negative tracking.
- Keep body, nav, labels, and buttons in a neutral UI sans with strong readability.
- Make hierarchy with size and spacing before increasing weight.
- Headline copy should be short and decisive. Body copy should be plainspoken and unforced.
- Favor titles that suggest clarity, retrieval, memory, or understanding over generic productivity slogans.

### Color

- Keep the action layer monochrome: near-black primary buttons on white.
- Use black, white, and neutral gray almost exclusively.
- If semantic status colors are necessary inside embedded product UI, keep them contained to the product fragment rather than the marketing shell.
- Keep dark surfaces scarce. The footer should usually be the main dark region.
- Prefer neutral warmth over stark blue-white minimalism.

### Components

- Feature cards: soft neutral surface, concise copy, minimal iconography, precise alignment.
- Product mockup cards: white or lightly framed containers with actual UI fragments inside.
- Quote cards: compact, credible, and understated. Use only when social proof actually helps.
- Pricing: standard light tiers plus one featured dark tier if emphasis is needed.
- Navigation: simple white top bar with one strong CTA and low visual noise.
- Knowledge surfaces should emphasize sidebars, search bars, article rows, node relationships, filters, citations, and document structure.
- Logos, integrations, and proof rows should feel structured and almost documentation-like, not decorative.

### Motion and depth

- Use subtle elevation only where it helps legibility.
- Prefer soft borders and faint shadows over dramatic lift.
- Avoid interaction-heavy flourish on marketing pages unless it supports the product story.
- Transitions should feel crisp and fast rather than bouncy.

## Before finalizing

Check for these failure modes:

- Too many dark cards.
- Any accent color leaking into CTAs, icons, or section backgrounds.
- Headline/body font boundary blurred.
- Cards feeling too playful, glossy, or rounded.
- Decorative mockups replacing actual product UI.
- Too much empty space without enough structural tension.
- Too much density without enough calm breathing room.
- Too many equally prominent actions in one viewport.
- Mobile layouts shrinking cards instead of reflowing them.

## Reference

Read [references/system.md](references/system.md) when you need the concrete monochrome token set, knowledge-product tone calibration, component guidance, spacing rhythm, or a reusable style vocabulary for implementation.
