# Research board design spike

Open `/spike/board-design` in the frontend dev server for the original visual reference.
Its approved presentation is now shared with the production `BoardPane` through
`ui/board-studio.css`. `/spike/board` exercises that production component with a
fixture content source and local persistence.

Production retains the existing saved shape schema and sync service. The rail creates
native sticky notes, text, frames, arrows, and ink; the library adds real workspace
documents, notes, instruments, and links. Existing tasks, excerpts, and two-sided cards
remain supported. No sample objects or instrument values are seeded into real boards.

## Direction

- Warm paper by default; a dedicated charcoal board mode with lighter cards and controls.
  Fine borders, restrained shadows, and
  small desktop controls with larger touch targets.
- One header, a stationary creation rail, and separate camera/history controls.
- Context appears on demand: select an object for actions; tap Pencil for its
  palette; open Library when adding material. Touching the canvas dismisses panels.
- Objects read as their content: a paper, a personal note, a discussion, a video,
  code, or an Aladin instrument. No permanent mini-toolbars on the cards.
- Human authorship first. No tutor sequence or persistent Copilot panel.

## Try

Move and resize objects; the labeled arrows remain attached. Shift-select objects
and group them. Draw with Pencil, make a connection, add text, or draw a frame.
Select a source and choose Open, or select a sticky note and choose Edit note.
Change its color, duplicate it, delete it, and undo. Search the Library and add a
sample object, insert an HTTP(S) URL, or paste text/links onto the canvas.

Keyboard: V select, H pan, P pen, A arrow, T text, F frame, N note, Escape dismiss.
The appearance button switches between light paper and the board's own dark palette
without resetting the document or undo history. Neither mode follows the app's theme.

## Deliberately simulated

All sources and instrument values are illustrative; no live market feed, real
workspace lookup, media player, PDF viewer, or link unfurling is connected. The
Open action previews/edits the local object's metadata and notes. Refreshing the
page restores the sample board. These limitations apply only to `/spike/board-design`,
not to the real board. Production instrument cards open their source artifact; they
do not display simulated market charts or claim to be live embedded instruments.

## References

- [Miro's simplified UI](https://help.miro.com/hc/en-us/articles/20967864443410-Miro-s-new-simplified-user-interface):
  separation of creation tools, board controls, and navigation.
- [Freeform for Mac](https://support.apple.com/guide/freeform/welcome/mac):
  a light spatial surface for mixed media, notes, and connected ideas.

These are interaction and visual references, not a pixel-for-pixel reproduction.
