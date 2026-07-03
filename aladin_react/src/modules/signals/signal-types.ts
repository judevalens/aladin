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

// The surface has two lenses. "inbox" is the shared fishing feed (discovered claims, served
// from the local sync cache). "book" is your own authored theses, each marked to market by how
// many discovered sources support (assertCount) vs contradict (denyCount) it — fetched over HTTP.
export type SignalLens = "inbox" | "book";
