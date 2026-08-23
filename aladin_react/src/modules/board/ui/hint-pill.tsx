/** The amber hint above the dock — copy comes from the caller. */
export function HintPill({ text }: { text: string }) {
  return (
    <div className="board-edge-above-dock pointer-events-none absolute left-1/2 -translate-x-1/2 whitespace-nowrap rounded-full border border-amber-line bg-amber-soft px-4 py-2 text-board-label text-amber">
      {text}
    </div>
  );
}
