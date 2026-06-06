# Aladin — Browser (Folders view) Spec

The **Browser** is the Folders view: a knowledge-file explorer with a nested tree, a cascading **Miller-column popup** for deep folders, a right-click context menu, and a tabbed editor. This is the precise build spec — every measurement is from the working prototype. Reference render: `/screens/02-folders-dark.png`. Tokens/components: see `DESIGN_SPEC.md`.

> Stack target: React + Tailwind + shadcn/lucide. The tree and Miller popup are **custom** (shadcn has no tree/Miller); use shadcn `ContextMenu` for the right-click menu and `ScrollArea` for scroll regions.

---

## 0. Data model

Tree nodes (see `TREE` in `ide.jsx`):
```ts
type Node = {
  id: string;
  type: 'folder' | 'page' | 'link' | 'voice' | 'file' | 'signal';
  name: string;
  meta?: string;          // e.g. "2.4 MB", "Edited 2h ago", "notion.so"
  signals?: number;       // amber signal-count chip
  unread?: boolean;       // amber dot
  children?: Node[];      // folders only
};
```
Build two indexes once: `NODE_BY_ID` and `PARENT_BY_ID`. Helpers:
- `pathOf(id)` → array of ancestor **names** (for breadcrumbs).
- `ancestorsOf(id)` → array of ancestor **nodes** (for seeding the Miller path).

Artifact type metadata (`TYPE`): `page/link/voice/file/signal` each → `{ label, icon, hue, letter }`. Folder icon is always `folder`; leaves use `TYPE[type].icon`.

---

## 1. Three-pane layout (Folders view)

`flex` row filling the body under the title bar:

1. **Explorer sidebar** — `w-[274px] shrink-0 bg-explorer border-r border-line flex flex-col`.
2. **Editor** — `flex-1 min-w-0 bg-[--edit] flex flex-col` (tab strip → breadcrumb → content → status bar).
3. Overlays (portaled): **Miller popup** (z-40), **Context menu** (z-50), **⌘K palette** (z-60).

---

## 2. Explorer sidebar

### 2a. Header row — `flex items-center gap-2 px-3 pt-[13px] pb-[11px]`
- `WORKSPACE` — mono 11, weight 700, letter-spacing 1, `text-ink-3`.
- spacer, then three 24×24 `rounded-md grid place-items-center` icon buttons (`.row` hover):
  - **add** (`Plus`, `text-ink-3`) → opens ⌘K.
  - **columns** (`Columns3`, `text-ink-2`) → opens the **Miller popup** anchored to this button's rect (no folder seed = starts at Workspace).
  - **sliders** (view options, `text-ink-3`) → no-op stub.

### 2b. Search pill — `px-3 pb-2.5`
`flex items-center gap-2 bg-field border border-line rounded-lg px-2.5 py-1.5 cursor-pointer`; `Search` icon (`text-ink-3`), "search or ask" (mono 12 `text-ink-3`), a `⌘K` kbd chip. Opens ⌘K.

### 2c. Scoped breadcrumb (only when the explorer is re-rooted via `scopeId`)
`flex items-center gap-[3px] px-2.5 pb-2 mono text-[10.5px] text-ink-3`: a back chevron (22×22, rotate 180°), a home icon (→ clear scope), optional parent (`…`/name), `/`, then current node name (bold, `max-w-[150px]` ellipsis). Most installs can defer scoping and rely on the Miller popup instead.

### 2d. Tree — `flex-1 overflow-auto px-2 pt-0.5 pb-3`
Recursive `TreeNode` rows (§3).

---

## 3. Tree node (recursive) — the drill rule lives here

Row container: `relative flex items-center gap-[7px] h-7 cursor-pointer rounded-md`, `paddingLeft: 10 + depth*15`, `pr-2.5`.

- **Active** (`node.id === sel`): `bg-[rgb(var(--sel))]`, `text-ink`, **plus a 2px amber left bar** (`absolute left-0 top-[5px] bottom-[5px] w-0.5 rounded bg-amber`).
- Inactive color: folders `text-ink`, leaves `text-ink-2`.
- **Chevron / spacer (14px slot):**
  - Inline-expandable folder → a 14×14 chevron, `text-ink-3`, **rotate 90° when open**, `transition-transform .12s`.
  - Drill folder or leaf → a blank 14px spacer (no chevron).
- **Type icon** — 16px slot; folder → `folder` icon (`text-ink-2`), leaf → `TYPE[type].icon` (`text-ink-3`).
- **Name** — `flex-1 text-[13px]` weight `600 active / 500 folder / 400 leaf`, `truncate`.
- **Trailing:** `signals` chip (mono 10 amber, `signal` icon + n); `unread` dot (6px amber); for folders, the **child count** (mono 10.5 `text-ink-4`) — and if it's a drill folder, also a chevron after the count (the drill affordance).
- **Children:** when an inline folder is open, render a nested container with a **vertical guide line** (`absolute w-px bg-line2` at `left: pad+6`, full height) and the children at `depth+1`.

### THE DRILL RULE (critical — agents get this wrong)
`MAX_INLINE = 2`. A folder **expands inline** only while `depth < 2` (i.e. depths 0 and 1; the tree shows **3 visible levels: 0, 1, 2**). A folder at **`depth >= 2` does NOT expand inline** — clicking it **drills in**: it opens the **Miller popup** (§4), anchored to the clicked row's bounding rect, seeded with that folder's ancestor path. This keeps indentation from marching off-screen. Leaves always open in the editor; inline folders toggle `openSet`.

```
onClick(node):
  if folder && depth >= MAX_INLINE  → openMiller(node.id, rowRect)   // drill
  else if folder                    → toggle(node.id) in openSet      // expand inline
  else                              → select(node.id) → open tab/editor
onContextMenu(node, e)              → open context menu at cursor (§5)
```

---

## 4. Miller-column popup (the deep-folder browser)

A **floating, fixed-size cascading column browser** that pops over the workspace. Triggered by: (a) clicking a drill folder (depth≥2), (b) the sidebar **columns** button, (c) context-menu **Browse/Reveal in columns**. This is NOT a modal — it's a popover-style menu.

### Shell
- Portal, z-40, with a **transparent full-viewport click-catcher** behind it (click outside or **Esc** → close).
- Pane: `position: fixed`, **fixed size** `width = min(216*4, vw-24)` (**= 864px → exactly 4 columns**), `height: 460`. It **never resizes as you browse** — going deeper than 4 columns scrolls horizontally instead.
- Anchored to the trigger: `left = anchorRect.right - 2`, `top = anchorRect.top - 4`, then **clamped** into the viewport (`left ∈ [12, vw-PANE_W-14]`, `top ∈ [12, vh-PANE_H-14]`).
- Style: `bg-explorer border border-line rounded-card`, shadow `0 22px 60px rgba(0,0,0,.62), 0 0 0 1px rgba(0,0,0,.4)`, `overflow-hidden`, `animate-[ixmenu]` (`from {opacity:0; translateY(-4px) scale(.97)}`, .15s), `transform-origin: top left`.

### Header — `flex items-center gap-2.5 px-3 py-2 border-b border-line2`
`Columns3` icon + a **live breadcrumb** (mono 10.5): `workspace` then `/ {name}` per path segment (current = `text-ink`, ancestors `text-ink-3`), `/ {leaf}` when a leaf is previewed; spacer; close (×).

### Columns — horizontal flex, `overflow-x-auto overflow-y-hidden`, `scroll-snap-type: x proximity`, smooth scroll
- **Column 0 is always "Workspace"** (icon `home`, items = `TREE`). Then one column per id in `path`: title = folder name, items = its children.
- Each column: `w-[216px] shrink-0 border-r border-line2 flex flex-col`, `scroll-snap-align: end`.
  - Column header: `flex items-center gap-[7px] px-3 py-2 border-b border-line2` — folder/home icon + title (12.5/600, truncate) + spacer + item count (mono 10 `text-ink-4`).
  - Column body: `flex-1 overflow-y-auto flex flex-col gap-px px-[7px] pt-[5px] pb-2` — a `MillerRow` per item.
- **Auto-scroll to newest:** on any `path`/`leaf` change, set `scrollLeft = scrollWidth` (so the freshest column is visible).

### MillerRow — `relative flex items-center gap-2.5 h-[30px] px-[9px] pl-[11px] rounded-chip cursor-pointer`
- **Selected** (folder: `path[ci]===id`; leaf: `leaf===id`) → `bg-amber-soft` + **2px amber left bar**, `text-ink`. Folder icon turns **amber** when selected.
- Icon 16px (folder / `TYPE[type].icon`), name 12.5 (weight 600 selected / 500 folder / 400 leaf, truncate), `signals`/`unread`, and for folders a trailing **child count + chevron**.
- **Click** a folder in column `ci` → `path = [...path.slice(0, ci), folder.id]` (replaces everything to the right, cascades a fresh column). **Click** a leaf → set `leaf` (opens preview column). **Double-click** a leaf → open it in the editor immediately.

### Leaf preview column (`w-[282px]`, appears when a leaf is selected) — `bg-card`
- Top `p-[18px]`: a **122px hatched thumbnail** (`rounded-[10px] border border-line`, `repeating-linear-gradient(135deg, field, field 10px, card 10px, card 20px)`) with the type icon (26px) centered.
- `TYPE.label` uppercase · meta (mono 10 `text-amber`), name (14.5/600), full path (mono 10.5 `text-ink-3`).
- spacer, then a full-width **"Open in editor"** button (`bg-amber text-[--onAccent] rounded-[9px] py-[9px]`, `ArrowRight` icon).

### Footer — `h-[30px] border-t border-line, mono 10 text-ink-4`
Hints: `click folder to expand →` · `double-click a file to open` · (spacer) · `esc to close`.

---

## 5. Right-click context menu

Use shadcn **`ContextMenu`** on every tree row (and Miller rows if you want). Restyle to: `bg-explorer border border-line rounded-[10px] p-[5px]`, shadow `0 18px 50px rgba(0,0,0,.62), 0 0 0 1px`, `animate-[ixmenu] .12s`, width ~212px. Open at cursor, clamp to viewport, close on outside-click / **Esc** / **scroll**.

Item row: `h-[30px] flex items-center gap-2.5 px-[9px] rounded-md`, icon + label (12.5) + optional kbd hint; **separators** `h-px bg-line2 my-[5px]`.

- **Folder menu:** **Browse in columns** (hero — amber icon, label weight 600, hint `↵`) → opens Miller seeded at this folder; **Expand/Collapse**; **New item here**; — ; **Rename**; **Delete** (danger, `text-against`).
- **File menu:** **Open** (hero); **Reveal in columns** (opens Miller seeded at the parent with this leaf pre-previewed); — ; **Rename**; **Delete** (danger).

---

## 6. Editor pane (right of the explorer)

- **Tab strip** — `h-10 bg-chrome border-b border-line flex`. Each tab: `flex items-center gap-2 px-[13px] max-w-[220px]`; active = `bg-[--edit]` + **2px amber top bar**, `text-ink`; inactive `text-ink-3`. Type icon + name (12.5, 600 active) + a close × that fades in on hover (`opacity-0 group-hover:opacity-100`). Right cluster: `Search / Star / Network / Sliders` icon buttons (28×28).
- **Breadcrumb** — `px-11 py-2 border-b border-line2 mono 11 text-ink-3`: `workspace / …pathOf(sel)… / {name}` (current `text-ink-2`).
- **Content** — `flex-1 overflow-auto`. Empty → centered "No artifact open · ⌘K to search". Else by type: **page** → document editor; **link** → an og:image preview card (150px hatched header + meta + title + blurb); **file** → a centered file card (46px icon tile + name + "meta · indexed by knowledge engine"); **signal** → the Signals push card.
- **Status bar** — `h-[26px] bg-chrome border-t border-line flex items-center gap-4 px-3.5 mono 10.5 text-ink-3`: type label (`text-ink-2`), word-count/meta, spacer, "● 3 signals today" (amber), "⌘K command".

---

## 7. State & wiring

```ts
const [openSet, setOpenSet]   = useState<Set<string>>(new Set(['product'])); // expanded inline folders
const [sel, setSel]           = useState('thesis');        // selected/open artifact
const [tabs, setTabs]         = useState(['thesis']);      // open editor tabs
const [scopeId, setScope]     = useState<string|null>(null); // optional re-root
const [miller, setMiller]     = useState<null | { seed: string[]; anchor: DOMRect; seedLeaf?: string }>(null);
const [ctx, setCtx]           = useState<null | { node: Node; x: number; y: number }>(null);

toggle(id)        // add/remove from openSet
onSel(id)         // setSel + add to tabs if absent
openMiller(folderId, anchorRect, seedLeaf?)
  // seed = folderId ? [...ancestorsOf(folderId).map(a=>a.id), folderId] : []
closeTab(id)      // remove; if it was sel, select the last remaining tab
```

`anchor` is the trigger row's `getBoundingClientRect()` — capture it in the row's `onClick`/`onContextMenu` (`e.currentTarget.getBoundingClientRect()`).

---

## 8. Behavior checklist (verify these — they're the precise bits)
- [ ] Folders at depth 0–1 expand inline with a rotating chevron + vertical guide line; depth ≥ 2 folders **drill into the Miller popup** instead (do not expand inline).
- [ ] Miller pane is a **fixed 864×460** popover anchored to the trigger, clamped to viewport, dismiss on outside-click/Esc — **not** a centered modal, **no** dimming scrim.
- [ ] Clicking a folder in column *n* truncates the path to *n* and cascades a fresh column; ancestors stay; horizontal scroll snaps to the newest column.
- [ ] Selecting a leaf opens a 282px preview column with "Open in editor"; double-clicking a leaf opens it directly.
- [ ] Right-click any row → context menu at the cursor; "Browse/Reveal in columns" is the hero action and opens the Miller popup.
- [ ] Selected rows (tree and Miller) show the 2px amber left bar; active editor tab shows the 2px amber top bar.
- [ ] Everything themes via tokens — works in both Dark and Soft.
