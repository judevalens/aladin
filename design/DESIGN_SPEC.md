# Aladin IDE — Design Spec (Tailwind + React + shadcn/ui)

This is the implementation spec for the Aladin IDE design, mapped to **React + Tailwind CSS + shadcn/ui + lucide-react**. Pair it with **`PRD.md`** (product context, data model, behavior). Reference renders are in **`/screens`**.

> Philosophy: dark-minimal, IDE-like, calm. Content leads; the knowledge graph stays out of the way until summoned. Build with shadcn primitives, restyle them to these tokens — don't ship default shadcn look.

---

## 1. Theme tokens → Tailwind + CSS variables

Two themes (**Dark** default, **Soft**). Drive both with CSS variables toggled by a `data-theme` attribute (or `.dark`/`.theme-soft` class). Map them into Tailwind via `hsl(var(--…))` or raw vars.

### `globals.css`
```css
@layer base {
  :root, [data-theme="dark"] {
    /* surfaces */
    --rail:#0b0b0e; --panel:#0f0f12; --bg:#0d0d10; --chrome:#0b0b0e;
    --field:#161619; --card:#121215; --raise:#17171c; --explorer:#101013;
    /* ink */
    --ink:#eceaef; --ink-2:#9694a0; --ink-3:#615f6b; --ink-4:#403e48;
    /* lines / states */
    --line:255 255 255 / 0.07; --line-2:255 255 255 / 0.045;
    --hover:255 255 255 / 0.035; --sel:255 255 255 / 0.06;
    /* accent */
    --amber:#c9925a; --amber-soft:201 146 90 / 0.12; --amber-line:201 146 90 / 0.30;
    /* semantic */
    --for:#5cba8f; --against:#d8796b; --catalyst:#5b9bd8; --echo:#9a8cd8; --neutral:#7f8aa0;
  }
  [data-theme="soft"] {
    --rail:#1a1a1d; --panel:#1e1e21; --bg:#202024; --chrome:#1a1a1d;
    --field:#242428; --card:#202023; --raise:#252529; --explorer:#1e1e21;
    --ink:#d7d6da; --ink-2:#9795a0; --ink-3:#67656f; --ink-4:#4a4952;
    --amber:#c69566; --amber-soft:198 149 102 / 0.13; --amber-line:198 149 102 / 0.30;
    /* lines/states/semantic inherit from :root */
  }
  body { background: var(--bg); color: var(--ink); }
}
```

### `tailwind.config.ts`
```ts
export default {
  darkMode: ["class"],
  theme: {
    extend: {
      colors: {
        rail:"var(--rail)", panel:"var(--panel)", bg:"var(--bg)", chrome:"var(--chrome)",
        field:"var(--field)", card:"var(--card)", raise:"var(--raise)", explorer:"var(--explorer)",
        ink:{ DEFAULT:"var(--ink)", 2:"var(--ink-2)", 3:"var(--ink-3)", 4:"var(--ink-4)" },
        amber:{ DEFAULT:"var(--amber)", soft:"rgb(var(--amber-soft))", line:"rgb(var(--amber-line))" },
        forc:"var(--for)", against:"var(--against)", catalyst:"var(--catalyst)", echo:"var(--echo)", neutral:"var(--neutral)",
      },
      borderColor:{ line:"rgb(var(--line))", line2:"rgb(var(--line-2))" },
      fontFamily:{
        display:["'Space Grotesk'","system-ui","sans-serif"],
        sans:["system-ui","-apple-system","'Segoe UI'","sans-serif"],
        mono:["'JetBrains Mono'","ui-monospace","'SF Mono'","monospace"],
      },
      borderRadius:{ chip:"7px", card:"12px", modal:"14px" },
      boxShadow:{
        panel:"-24px 0 60px rgba(0,0,0,.5)",
        modal:"0 28px 70px rgba(0,0,0,.6)",
        toast:"0 12px 40px rgba(0,0,0,.5)",
      },
      keyframes:{
        pop:{ from:{opacity:"0",transform:"translateY(8px) scale(.985)"}, to:{opacity:"1",transform:"none"} },
        slidein:{ from:{transform:"translateX(30px)",opacity:"0"}, to:{transform:"none",opacity:"1"} },
        cardin:{ from:{opacity:"0",transform:"translateY(8px)"}, to:{opacity:"1",transform:"none"} },
        shimmer:{ "0%,100%":{opacity:".5"}, "50%":{opacity:"1"} },
        pulse2:{ "0%,100%":{opacity:"1"}, "50%":{opacity:".4"} },
      },
      animation:{
        pop:"pop .17s cubic-bezier(.2,.8,.2,1)",
        slidein:"slidein .26s cubic-bezier(.2,.8,.2,1)",
        cardin:"cardin .26s cubic-bezier(.2,.8,.2,1)",
        shimmer:"shimmer 1.2s ease infinite",
        pulse2:"pulse2 1.8s ease infinite",
      },
    },
  },
};
```

Map shadcn's own tokens (`--background`, `--foreground`, `--popover`, `--border`, `--ring`, `--primary`) to these: `--background:var(--bg)`, `--popover:var(--explorer)`, `--border:rgb(var(--line))`, `--primary:var(--amber)`, `--ring:var(--amber)`, etc., so shadcn components inherit the theme.

### Fonts
`next/font` or `@fontsource`: **Space Grotesk** (display), **JetBrains Mono** (labels/meta). Body = system sans. Mono is used heavily for labels, handles, counts, hints, timestamps — treat it as a first-class UI font, ~9.5–11px, often uppercase with letter-spacing 0.6–0.8.

---

## 2. shadcn component map

| UI element | shadcn / lib | Notes |
|---|---|---|
| ⌘K command palette + Ask mode | **`Command` / `CommandDialog`** (cmdk) | Extend with an "ask" mode (see §6). Bind Cmd/Ctrl+K. |
| Drill-in detail panel (right slide-in) | **`Sheet`** (`side="right"`) | width `min(440px,100%)`, `shadow-panel`, `animate-slidein`. |
| Graph modal, Brief history | **`Dialog`** | centered, `rounded-modal shadow-modal animate-pop`, scrim blur. |
| Right-click menu on tree rows | **`ContextMenu`** | hero item "Browse in columns" opens the Miller popover. |
| Miller-column popover | **`Popover`** (custom content) | fixed 864px width = 4 columns; anchored to trigger. |
| Feed filter (All/News/Social/Email), Top/Latest | **`Tabs`** or **`ToggleGroup`** | pill style, selected = `bg-[rgb(var(--sel))]` + `border-line`. |
| Rail items, pills | **`Button`** (`variant="ghost"`) + **`Tooltip`** | rail icons 38×38, `rounded-[9px]`. |
| Avatars (social/email) | **`Avatar`** | initials, bg hue derived from name hash. |
| Stance pills, NEWSLETTER/REPLY labels | **`Badge`** (custom variants) | SUPPORTS=for, COUNTERS=against, CONTEXT=neutral. |
| "graph grew" confirmation | **`Sonner`** toast | bottom-center, `bg-raise border-amber-line`, subtle. |
| Scroll regions | **`ScrollArea`** | feed, panel body, command list. |
| Icons | **lucide-react** | mapping in §7. |

---

## 3. App shell layout

Fixed full-viewport, `flex flex-col`, `bg-bg text-ink font-sans overflow-hidden`.

- **Title bar** — `h-10 shrink-0 bg-chrome border-b border-line flex items-center px-3.5 gap-2`. Left: 3 traffic-light dots (12px). Center: a ⌘K command-bar pill (`bg-field border border-line rounded-chip`, mono 11, shows current context + a `⌘K` kbd chip).
- **Body** — `flex-1 flex min-h-0`:
  - **Activity rail** — `w-[52px] shrink-0 bg-rail border-r border-line flex flex-col items-center py-3`. Amber "A" logo (`size-8 rounded-[9px] bg-amber text-[#0f0f12] font-display font-bold grid place-items-center`, also opens ⌘K). Then rail buttons (Home, Folders, Signals, Sources, Graph) — `size-[38px] rounded-[9px] grid place-items-center`, active = `bg-[rgb(var(--sel))] text-ink` else `text-ink-3`; wrap each in a `Tooltip`. Signals shows a small amber unread dot. Command icon pinned bottom (`mt-auto`).
  - **Main** — `flex-1 min-w-0 overflow-auto`. Home renders its own header (no top bar). Other views render a `h-[46px] shrink-0 border-b border-line` top bar.

`.row` hover utility: add a class `hover:bg-[rgb(var(--hover))]` to interactive rows.

---

## 4. Home (Consume dashboard)

Scroll container; inner `max-w-[1080px] mx-auto px-7 pt-6 pb-15`. See `/screens/01-home-dark.png` and `/screens/06-home-soft.png`.

- **Header** — `flex items-end gap-3.5 mb-4`. Greeting `font-display text-[26px] font-semibold tracking-[-0.5px]`. Right cluster `flex items-center gap-3 shrink-0` with **every span `whitespace-nowrap shrink-0`** (this prevents the wrapping bug): a "updated {time} · 8 sources" (mono 10.5, `text-ink-4`, leading pulse dot `bg-forc animate-pulse2`) and a "Capture ⌘K" pill (mono 11, `bg-field border border-line rounded-chip px-2.5 py-1.5`, amber `Plus` icon).
- **Brief card** — `bg-panel border border-line rounded-card p-4 mb-6 flex gap-3`. Amber `Sparkles` icon. Body: a row with `YOUR BRIEF` (mono 9.5/700 uppercase `text-amber whitespace-nowrap`), spacer, and an "earlier briefs" pill (`Dialog` trigger, mono 10, `Clock` icon, `whitespace-nowrap shrink-0`). Brief text `text-sm leading-relaxed text-ink-2 mt-1.5`.
- **Two-column** — `flex gap-[22px] items-start`:
  - **Feed** (`flex:1.7`, `min-w-0`): "Top for you" (`font-display text-[15px] font-semibold`) + a filter `Tabs`. Then time-bucket groups (`Just now / This morning / Earlier`, mono 10 uppercase labels) of cards (`flex flex-col gap-2.5`).
  - **Rail** (`flex:1`, `min-w-[248px]`, `sticky top-0 flex flex-col gap-4`): **Up Next** widget (catalysts: a 42px date block + title + "in N days · TAG"), **Tracking** widget (rows: type icon + name + "N new" mono + trend arrow; click filters feed → show a removable topic chip in the feed header), **Pinned** widget (conditional).
- **Feed cards** — base `bg-card border border-line rounded-card p-3.5 cursor-pointer animate-cardin`; `hover:bg-raise hover:border-ink-4`, pinned → `border-amber-line`. Pin/dismiss icon buttons (`size-[26px] rounded-chip`) appear top-right on hover (`opacity-0 group-hover:opacity-100`). Footer on every card: a faint "✦ why this" hint (mono 10) + a faint graph glyph + connection count.
  - **Social**: `Avatar` 34px, name 13.5/600, platform icon, `@handle · time` mono 11, post 13.5 leading-[1.55] `text-ink-2`, engagement ♡/↻ mono 10.5.
  - **Email**: avatar, `Mail` icon, sender 13/600, optional `Badge` (NEWSLETTER/REPLY mono 8.5 uppercase), time right, subject 13.5/600, snippet 12.5 `text-ink-3`.
  - **News**: outlet row (`Newspaper` icon + outlet mono 11/600 + time), headline `font-display text-[15px] font-semibold tracking-[-0.2px]`, dek 12.5 `text-ink-3`, + 76×76 thumbnail (`rounded-[9px]`, diagonal gradient of the outlet hue, outlet initial centered).

---

## 5. Drill-in panel + graph modal (graph on demand)

- **Detail panel** = shadcn **`Sheet`** (`side="right"`), `w-[min(440px,100%)] bg-panel border-l border-line shadow-panel animate-slidein`; scrim `bg-[rgba(6,6,8,.5)] backdrop-blur-[2px]`.
  - Header: source icon + UPPERCASE source label (mono 11/700) + close. When pivoted, a "back" control replaces the icon.
  - Body (`ScrollArea`): full item; action row (**Pin** primary-amber when active / **Open original** ghost / dismiss icon); amber **"ALADIN SUGGESTS"** block (`bg-amber-soft border border-amber-line rounded-[10px]`, "Link this to {thesis} as evidence?" + **Accept**/Not now); **"HOW THIS CONNECTS"** header + "Open in graph" button; sections **Mentions** (entity chips), **Linked topics** (thesis rows + "Connect to something" by-hand adder), **Echoes something you made** (violet `border-echo` note block), **Also in your feed** (related rows).
  - **Explorable:** clicking any node re-centers the panel on it (entity/thesis/note view) with a back trail. Accept/Connect writes a claim and fires a **Sonner** "graph grew" toast.
- **Graph modal** = `Dialog`. Radial node-link: focus node centered, neighbors on a 168px-radius circle, SVG connector lines colored per node type. Header (GRAPH label, back-when-pivoted, node count); footer legend (Topic=amber / Entity / Your note=echo / Feed item) + "click a node to pull the thread". Clicking a neighbor pivots.
- **Brief history** = `Dialog`, briefs grouped by date as a timeline, latest amber-accented.

---

## 6. ⌘K Command Palette + Ask-my-graph

Build on shadcn **`CommandDialog`** (cmdk), restyled: box `w-[min(620px,100%-80px)] bg-explorer border border-line rounded-modal shadow-modal animate-pop`, scrim blur. See `PRD.md` §4 for behavior.

- **List mode:** `CommandInput` (icon turns amber + `Sparkles` when the query looks like a question), `CommandGroup`s (Create / Go to / Jump to / **Ask your graph**), `CommandItem`s with icon + label + optional sub + kbd hint. Selected item = `bg-amber-soft` + a 2px amber left bar. Empty input shows ~4 **suggested questions** (cold start). Footer: ↑↓ navigate · ↵ open · ⌘K hint.
- **Question detection** (`isQuestion`): ends with `?`, OR starts with what/how/why/which/who/when/is/are/can/should/summarize/compare/connect/tell me/show me, OR ≥4 words and not a command verb. When true, surface an "Ask your graph" item at the top (hint `↵ ask`).
- **Ask mode** (replaces list content; box grows to `h-[min(560px,78vh)]`):
  - Header: back arrow + amber `Sparkles` + the question + esc.
  - **Answer card** (`bg-amber-soft border border-amber-line rounded-card p-3.5`): mono amber `ANSWER` + optional `LIVE` badge + intent badge (RECALL/DISCOVERY/TEMPORAL); answer `text-sm leading-[1.65]`. Loading = 3 `animate-shimmer` bars.
  - **"FROM YOUR GRAPH"** list: clickable source rows (icon + label + sub + optional stance `Badge` + arrow) → open graph modal on that node.
  - Footer: "Ask a follow-up…" input (Enter asks; threaded) + a "Save" pill (writes answer back to graph).
  - Esc: ask view → back to list; list → close.
- **Engine** (`askGraph`, port from `graph-qa.jsx`): classify intent (recall/discovery/temporal) → resolve entities by alias → build a grounded answer + the exact source nodes → optional LLM prose polish over the grounding. Keep the deterministic builder as the offline fallback and as a test oracle.

---

## 7. Icon mapping (lucide-react)

The prototype uses inline SVG paths; map to lucide:

| Aladin | lucide | | Aladin | lucide |
|---|---|---|---|---|
| capture/plus | `Plus` | | home | `House` |
| inbox | `Inbox` | | thesis | `Hexagon`/`Gem` |
| entity | `Share2` | | graph | `Network` |
| spark | `Sparkles` | | command | `Command` |
| search | `Search` | | arrow | `ArrowRight` |
| close | `X` | | check | `Check` |
| clock | `Clock` | | save | `Bookmark` |
| mail | `Mail` | | news | `Newspaper` |
| mic (voice) | `Mic` | | source/file | `FileText` |
| link | `Link2` | | calendar | `Calendar` |
| up/down/flat (trend) | `TrendingUp`/`TrendingDown`/`Minus` | | columns (Miller) | `Columns3` |
| asset | `CircleDollarSign` | | company | `Building2` |
| person | `User` | | concept | `CircleHelp` |
| x/discord/slack/telegram | brand glyphs or `AtSign` fallback | | | |

Default stroke ~1.7, sizes 12–17px depending on context.

---

## 8. Build order
1. Theme tokens + Tailwind config + fonts + shadcn init (restyle base tokens to match).
2. App shell (title bar, rail with tooltips, view router).
3. Home dashboard (header, brief, feed cards, rail widgets) — static first.
4. Drill-in `Sheet` panel + graph `Dialog` modal (graph-on-demand).
5. ⌘K `CommandDialog` (list mode) → add Ask mode + the QA engine.
6. Folders (tree + Miller `Popover` + `ContextMenu`), Signals, Sources, Graph views.
7. Wire the real graph store (persisted) + LLM provider; keep deterministic fallbacks.

## 9. Don'ts
- Don't ship default shadcn styling — restyle to the tokens (near-black surfaces, amber accent, mono labels).
- Don't use `whitespace`-default on header meta rows (caused wrapping). Use `whitespace-nowrap shrink-0`.
- Don't let a render error blank the app — add an error boundary.
- Don't hardcode two themes as two builds — one component tree, `data-theme` switch.
- No gradient-soup, no emoji, no rounded-corner-+-left-accent cliché. Calm and minimal.
