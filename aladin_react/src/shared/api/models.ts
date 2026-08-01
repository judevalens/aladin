export interface AuthRequest {
  email: string;
  password: string;
}

export interface AuthUser {
  id: string;
  email: string;
}

export interface AuthResponse {
  user: AuthUser;
  token?: string;
  expiresAt?: string | null;
}

export interface IntegrationToken {
  id: string;
  name: string;
  scopes: string[];
  status: string;
  expiresAt?: string | null;
  createdAt: string;
  lastUsedAt?: string | null;
}

export interface IntegrationTokenListResponse {
  tokens: IntegrationToken[];
}

export interface IntegrationTokenCreateRequest {
  name: string;
  scopes: string[];
  expiresAt?: string | null;
}

export interface CreatedIntegrationToken {
  token: string;
  integrationToken: IntegrationToken;
}

export interface Source {
  id: string;
  name: string;
  type: string;
  syncMode: string;
  syncState: string;
  config: Record<string, unknown>;
  autoPromoteThreshold: number;
  suggestThreshold: number;
  createdAt: string;
  lastSyncedAt?: string | null;
}

export interface SourceCreateRequest {
  kind: string;
  name?: string | null;
  subreddit?: string | null;
  feed?: string | null;
  minScore?: number | null;
  includeComments?: boolean | null;
  topComments?: number | null;
  sort?: string | null;
  query?: string | null;
  handle?: string | null;
  limit?: number | null;
  minLikes?: number | null;
  maxResults?: number | null;
}

export interface ProviderConnectionProvider {
  provider: string;
  label: string;
  backend: string;
  providerConfigKey: string;
  description: string;
  category: string;
  capabilities: string[];
  comingSoon: boolean;
  available: boolean;
  connected: boolean;
  connectionId?: string | null;
  grantedScopes: string[];
}

export interface ProviderConnectionProvidersResponse {
  providers: ProviderConnectionProvider[];
}

export interface ProviderConnectSession {
  connectSessionToken: string;
  connectLink: string;
  expiresAt?: string | null;
  nangoBaseUrl: string;
  nangoConnectBaseUrl: string;
}

export interface UserArtifact {
  id: string;
  type: string;
  folderId?: string | null;
  title: string;
  content: string;
  summary?: string | null;
  sourceUrl?: string | null;
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface UserArtifactCreateRequest {
  type?: string;
  folderId?: string | null;
  title?: string;
  content?: string;
  summary?: string | null;
  sourceUrl?: string | null;
  metadata?: Record<string, unknown>;
}

export interface UserArtifactUpdateRequest {
  type?: string | null;
  folderId?: string | null;
  title?: string | null;
  content?: string | null;
  summary?: string | null;
  sourceUrl?: string | null;
  metadata?: Record<string, unknown> | null;
}

// BlockNoteDocument is a JSON array of BlockNote blocks. We don't model the
// inner block shape here — the editor enforces it. Treating it as unknown[]
// at the boundary keeps the wire contract honest without forcing the rest
// of the app to depend on @blocknote/core types.
export type BlockNoteDocument = unknown[];

export interface PageDocumentRecord {
  id: string;
  title: string;
  blocks: BlockNoteDocument;
  revision: number;
  updatedAt: string;
}

export interface PageSaveRequest {
  blocks: BlockNoteDocument;
  revision: number;
}

export interface UploadedFileRecord {
  id: string;
  url: string;
  uploadedAt: string;
}

export interface FolderRecord {
  id: string;
  parentId?: string | null;
  title: string;
}

export interface BrowserTreeRecord {
  id: string;
  parentId?: string | null;
  kind: string;
  title: string;
  artifactId?: string | null;
  artifactType?: string | null;
  updatedAt?: string | null;
  children: BrowserTreeRecord[];
}

export interface FolderCreateRequest {
  title: string;
  parentId?: string | null;
}

export interface FolderUpdateRequest {
  title: string;
}

export interface BreadcrumbRecord {
  id?: string | null;
  label: string;
}

export type ArtifactKind = "note" | "link" | "voice" | "file" | "app";

/** One ticker-search hit — a security (e.g. NVDA), the command-box typeahead row. */
export interface InstrumentHit {
  id: string;
  symbol: string;
  name: string;
  exchange: string;
  assetClass: string;
  isActive: boolean;
}

/** One global-search result row. `kind` drives the client's icon + route. */
export interface SearchHit {
  kind: string; // ticker | company | person | entity | page | shard
  id: string;
  title: string;
  subtitle?: string;
  score: number;
}

/** One typed group of search hits (backend-ordered). */
export interface SearchSection {
  type: string; // entity | artifact
  label: string;
  hits: SearchHit[];
}

/** GET /api/search — the whole federated, sectioned command-box payload. */
export interface SearchResponse {
  sections: SearchSection[];
}

/** One OHLCV bar from GET /api/instruments/{symbol}/bars (raw/unadjusted). */
export interface Bar {
  t: string; // RFC-3339 bar start
  o: number;
  h: number;
  l: number;
  c: number;
  v: number;
}

/** One tracked security on the Markets watchlist. */
export interface WatchlistItem {
  instrumentId: string;
  symbol: string;
  name: string;
  exchange: string;
  addedAt: string;
}

export type ArtifactPropertyType = "text" | "number" | "date" | "select" | "url" | "tags";

/** One typed user-defined property on an artifact (stored under metadata.properties). */
export interface ArtifactProperty {
  key: string;
  type: ArtifactPropertyType;
  /** Scalar value for text/number/date/select/url. */
  value: string;
  /** Chips for the `tags` (multi-value) type. */
  values?: string[];
  /** Choices for `select`-typed properties. */
  options?: string[];
}

export interface Artifact {
  id: string;
  folderId?: string | null;
  title: string;
  content: string;
  summary?: string | null;
  kind: ArtifactKind;
  updatedLabel: string;
  sourceUrl?: string | null;
  resourceUrl?: string | null;
  properties?: ArtifactProperty[] | null;
}

export interface FolderNode {
  id: string;
  parentId?: string | null;
  title: string;
}

export interface ArtifactPreview {
  id: string;
  title: string;
  kind: ArtifactKind;
  updatedLabel?: string | null;
}

/**
 * Tree node kinds. `research` is a research folder (RESEARCH_SURFACE_PRD §5) — a
 * CONTAINER like `folder`, with its own strategy extension. Use isContainerKind()
 * rather than comparing to "folder" so research participates in expand/drill/move.
 */
export type BrowserNodeKind = "folder" | "artifact" | "research";

/** A research folder's light strategy fields, carried on its sync frame (§5). */
export interface ResearchNodeMeta {
  runState: string;
  execMode: string;
  sourceKind: string;
}

export interface BrowserTreeNode {
  id: string;
  parentId?: string | null;
  kind: BrowserNodeKind;
  title: string;
  artifactId?: string | null;
  artifactPreview?: ArtifactPreview | null;
  /** Set on kind === "research" only. */
  research?: ResearchNodeMeta | null;
  children: BrowserTreeNode[];
}

export interface BreadcrumbItem {
  id?: string | null;
  label: string;
}

export interface VoiceCaptureDraft {
  folderId?: string | null;
  title: string;
  description: string;
  phase: "ready" | "recording" | "review" | "saving" | "failed";
  audioBlob?: Blob | null;
  audioUrl?: string | null;
  errorMessage?: string | null;
}
