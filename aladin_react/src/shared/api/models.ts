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

export interface PageDocumentRecord {
  id: string;
  title: string;
  content: string;
  revision: number;
  updatedAt: string;
}

export interface PageSaveRequest {
  content: string;
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

export type ArtifactKind = "note" | "link" | "voice" | "file";

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

export type BrowserNodeKind = "folder" | "artifact";

export interface BrowserTreeNode {
  id: string;
  parentId?: string | null;
  kind: BrowserNodeKind;
  title: string;
  artifactId?: string | null;
  artifactPreview?: ArtifactPreview | null;
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
