/** The amber hint above the dock (bottom 88px, centered) — copy comes from the caller. */
export function HintPill({ text }: { text: string }) {
  return (
    <div className="pointer-events-none absolute bottom-[calc(88px+var(--host-bottom-inset,0px))] left-1/2 -translate-x-1/2 whitespace-nowrap rounded-full border border-amber-line bg-amber-soft px-4 py-[9px] text-[14px] text-amber">
      {text}
    </div>
  );
}
