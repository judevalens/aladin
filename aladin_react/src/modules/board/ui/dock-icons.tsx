/** The dock's inline 24px stroke icons — paths verbatim from the design handoff. */
export function DockIcon({
  d,
  size = 21,
  strokeWidth = 1.8,
}: {
  d: string;
  size?: number;
  strokeWidth?: number;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d={d} />
    </svg>
  );
}

export const DOCK_PATHS = {
  insert: "M12 5v14M5 12h14",
  select: "M5 3l14 8-6 1.6L10 19z",
  pencil: "M4 20l1-4L16 5l3 3L8 19z",
  arrow: "M9 15l6-6M8.5 7.5a3.5 3.5 0 1 1 5 5l-1.5 1.5M15.5 16.5a3.5 3.5 0 1 1-5-5",
  pen: "M12 19l7-7-3-3-7 7-1 4zM18 8l1.5-1.5a2.1 2.1 0 0 0-3-3L15 5",
  highlighter: "M9 11l7-7 4 4-7 7zM9 11l-2 5 5.5-1.5M4 21h9",
  eraser: "M3 15a2 2 0 0 1 0-3l9-9 8 8-9 9H8zM20 20h-9",
  lasso: "M20 10c0 3-3.6 5.5-8 5.5S4 13 4 10s3.6-5.5 8-5.5 8 2.5 8 5.5zM8.5 15c-1.5 1.5-.5 3.5 1.5 4",
  zoomIn: "M12 5v14M5 12h14",
  zoomOut: "M5 12h14",
  trash: "M4 7h16M9 7V5h6v2M6 7l1 13h10l1-13",
  file: "M14 3v5h5M19 8v11a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7z",
  note: "M4 4h16v16H4zM8 9h8M8 13h8M8 17h5",
  link: "M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1.5 1.5M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1.5-1.5",
  search: "M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14M16 16l5 5",
  close: "M18 6L6 18M6 6l12 12",
  clipboard: "M8 8h13v13H8zM16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3",
} as const;
