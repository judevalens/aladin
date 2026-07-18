export interface ArtifactRow {
  id: string;
  folderId: string | null;
  title: string;
  kind: string;
  content: string | null;
  sourceUrl: string | null;
  resourceUrl: string | null;
  summary: string | null;
  metadataJson: string | null;
  updatedAt: number;
  syncStatus: string;
  version: number;
}

export interface BrowserNodeRow {
  id: string;
  parentId: string | null;
  title: string;
  kind: string;
  artifactId: string | null;
  updatedAt: number;
  syncStatus: string;
  version: number;
}

export interface BrowserNodeCreateResult {
  node: BrowserNodeRow;
  artifact?: ArtifactRow | null;
}

/**
 * Data-layer redesign: a row of the unified local `nodes` tree (one row per
 * folder/artifact). Mirrors the Rust NodeRow and the sync feed's per-field
 * columns; this is the authoritative local read model (converged by pull).
 */
export interface NodeRow {
  id: string;
  kind: string;
  parentId: string | null;
  position: number;
  title: string | null;
  artifactType: string | null;
  content: string | null;
  sourceUrl: string | null;
  summary: string | null;
  metadataJson: string | null;
  updatedAt: number;
}

export interface PageMetadataRow {
  id: string;
  title: string;
  revision: number;
  updatedAt: number;
  syncStatus: string;
  version: number;
}

export interface LocalBrowserNodeCreateInput {
  id: string;
  parentId: string | null;
  kind: string;
  title: string;
  artifactType?: string | null;
  content?: string | null;
  summary?: string | null;
  sourceUrl?: string | null;
  updatedAt: number;
  mutationId: string;
}

export interface LocalBrowserMutationInput {
  id: string;
  parentId: string | null;
  title: string;
  updatedAt: number;
  mutationId: string;
}

export interface LocalDeleteInput {
  id: string;
  updatedAt: number;
  mutationId: string;
}

export interface LocalArtifactMutationInput {
  id: string;
  folderId?: string | null;
  type?: string | null;
  title: string;
  content?: string | null;
  summary?: string | null;
  sourceUrl?: string | null;
  updatedAt: number;
  mutationId: string;
}

