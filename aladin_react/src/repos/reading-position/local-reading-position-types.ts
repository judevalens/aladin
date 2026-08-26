/** The local `reading_positions` replica row — id IS the artifact id. */
export interface LocalReadingPosition {
  id: string;
  page: number;
  /** The SERVER's unix-ms stamp (from the frame), not a local clock. */
  updatedAt: number;
}
