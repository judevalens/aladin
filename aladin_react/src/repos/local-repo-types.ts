export interface ArtifactRow {
  id: string;
  folderId: string | null;
  title: string;
  kind: string;
  content: string | null;
  sourceUrl: string | null;
  resourceUrl: string | null;
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

export interface PageContentRow {
  id: string;
  /** JSON-encoded BlockNote document. Parsed on the way out of the repo. */
  blocks: string;
  revision: number;
  updatedAt: number;
  syncStatus: string;
  version: number;
}

export interface LocalPageContentSaveInput {
  id: string;
  /** JSON-encoded BlockNote document. */
  blocks: string;
  revision: number;
  updatedAt: number;
  mutationId: string;
}
