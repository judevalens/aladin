// A signal is a claim — a contestable, entity-grounded proposition — rendered as a feed card.
// (Enriched records may be mixed into this surface later.)
export interface ClaimSubjectRef {
  id: string;
  name: string;
}

export interface ClaimSignal {
  id: string;
  text: string;
  polarity: string;
  trustTier: string;
  subjects: ClaimSubjectRef[];
  assertCount: number;
  denyCount: number;
  supports: number;
  contradicts: number;
  qualifies: number;
  signalScore: number;
  createdAt?: string;
}

export type SignalSort = "recent" | "top";
