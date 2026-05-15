import type {
  Artifact,
  ArtifactKind,
  AuthRequest,
  AuthResponse,
  BrowserTreeNode,
  BrowserTreeRecord,
  BreadcrumbItem,
  BreadcrumbRecord,
  CreatedIntegrationToken,
  FolderCreateRequest,
  FolderNode,
  FolderRecord,
  FolderUpdateRequest,
  IntegrationTokenCreateRequest,
  IntegrationTokenListResponse,
  PageDocumentRecord,
  PageSaveRequest,
  ProviderConnectSession,
  ProviderConnectionProvider,
  ProviderConnectionProvidersResponse,
  Source,
  SourceCreateRequest,
  UploadedFileRecord,
  UserArtifact,
  UserArtifactCreateRequest,
  UserArtifactUpdateRequest,
} from "@/shared/api/models";

export class ApiError extends Error {
  status?: number;
  statusText?: string;

  constructor(message: string, status?: number, statusText?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.statusText = statusText;
  }
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      message = body.error ?? body.message ?? message;
    } catch {
      // ignore
    }
    throw new ApiError(message, response.status, response.statusText);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });
  return parseResponse<T>(response);
}

function artifactKind(type?: string | null): ArtifactKind {
  switch ((type ?? "").toLowerCase()) {
    case "page":
    case "note":
      return "note";
    case "link":
      return "link";
    case "voice":
      return "voice";
    case "file":
      return "file";
    default:
      return "note";
  }
}

function metadataString(metadata: Record<string, unknown>, key: string) {
  const value = metadata[key];
  return typeof value === "string" ? value : undefined;
}

export function userArtifactResourceUrl(id: string) {
  return `/api/artifacts/${id}/resource`;
}

export function toArtifact(record: UserArtifact): Artifact {
  const kind = artifactKind(record.type);
  const title =
    record.title ||
    metadataString(record.metadata, "originalFilename") ||
    metadataString(record.metadata, "storageKey") ||
    "Untitled artifact";

  return {
    id: record.id,
    folderId: record.folderId,
    title,
    content: record.content,
    summary: record.summary,
    kind,
    updatedLabel: record.updatedAt,
    sourceUrl: record.sourceUrl,
    resourceUrl: kind === "voice" || kind === "file" ? userArtifactResourceUrl(record.id) : undefined,
  };
}

export function toBrowserTreeNode(record: BrowserTreeRecord): BrowserTreeNode {
  const kind = record.kind.toLowerCase() === "folder" ? "folder" : "artifact";
  return {
    id: record.id,
    parentId: record.parentId,
    kind,
    title: record.title,
    artifactId: record.artifactId,
    artifactPreview:
      kind === "artifact" && record.artifactId
        ? {
            id: record.artifactId,
            title: record.title,
            kind: artifactKind(record.artifactType),
            updatedLabel: record.updatedAt,
          }
        : undefined,
    children: record.children.map(toBrowserTreeNode),
  };
}

export const api = {
  register(req: AuthRequest) {
    return apiFetch<AuthResponse>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
  login(req: AuthRequest) {
    return apiFetch<AuthResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
  logout() {
    return apiFetch<void>("/api/auth/logout", { method: "POST" });
  },
  me() {
    return apiFetch<AuthResponse>("/api/auth/me");
  },
  getIntegrationTokens() {
    return apiFetch<IntegrationTokenListResponse>("/api/integration-tokens");
  },
  createIntegrationToken(req: IntegrationTokenCreateRequest) {
    return apiFetch<CreatedIntegrationToken>("/api/integration-tokens", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
  revokeIntegrationToken(id: string) {
    return apiFetch<void>(`/api/integration-tokens/${id}/revoke`, { method: "POST" });
  },
  getSources() {
    return apiFetch<Source[]>("/api/sources/");
  },
  createSource(req: SourceCreateRequest) {
    return apiFetch<Source>("/api/sources/", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
  deleteSource(id: string) {
    return apiFetch<void>(`/api/sources/${id}`, { method: "DELETE" });
  },
  getProviderConnectionProviders() {
    return apiFetch<ProviderConnectionProvidersResponse>("/api/provider-connections/providers");
  },
  startProviderConnect(provider: string) {
    return apiFetch<ProviderConnectSession>(`/api/provider-connections/${provider}/connect`, {
      method: "POST",
    });
  },
  syncProviderConnections() {
    return apiFetch("/api/provider-connections/sync", {
      method: "POST",
    });
  },
  disconnectProviderConnection(id: string) {
    return apiFetch<void>(`/api/provider-connections/${id}/disconnect`, { method: "POST" });
  },
  getFolders(parentId?: string | null) {
    const search = new URLSearchParams();
    if (parentId) search.set("parentId", parentId);
    return apiFetch<FolderRecord[]>(`/api/folders/${search.size > 0 ? `?${search}` : ""}`);
  },
  getFolder(id: string) {
    return apiFetch<FolderRecord>(`/api/folders/${id}`);
  },
  getFolderBreadcrumbs(id: string) {
    return apiFetch<BreadcrumbRecord[]>(`/api/folders/${id}/breadcrumbs`);
  },
  createFolder(req: FolderCreateRequest) {
    return apiFetch<FolderRecord>("/api/folders/", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
  updateFolder(id: string, req: FolderUpdateRequest) {
    return apiFetch<FolderRecord>(`/api/folders/${id}`, {
      method: "PATCH",
      body: JSON.stringify(req),
    });
  },
  getBrowserTree() {
    return apiFetch<BrowserTreeRecord[]>("/api/browser/tree");
  },
  getUserArtifacts(folderId?: string | null) {
    const search = new URLSearchParams();
    if (folderId) search.set("folderId", folderId);
    return apiFetch<UserArtifact[]>(`/api/artifacts/${search.size > 0 ? `?${search}` : ""}`);
  },
  getUserArtifact(id: string) {
    return apiFetch<UserArtifact>(`/api/artifacts/${id}`);
  },
  createUserArtifact(req: UserArtifactCreateRequest) {
    return apiFetch<UserArtifact>("/api/artifacts/", {
      method: "POST",
      body: JSON.stringify(req),
    });
  },
  updateUserArtifact(id: string, req: UserArtifactUpdateRequest) {
    return apiFetch<UserArtifact>(`/api/artifacts/${id}`, {
      method: "PATCH",
      body: JSON.stringify(req),
    });
  },
  async uploadUserArtifact(input: {
    type: string;
    file: File | Blob;
    filename: string;
    title?: string;
    summary?: string;
    folderId?: string | null;
  }) {
    const formData = new FormData();
    formData.append("type", input.type);
    formData.append("file", input.file, input.filename);
    if (input.title) formData.append("title", input.title);
    if (input.summary) formData.append("summary", input.summary);
    if (input.folderId) formData.append("folderId", input.folderId);

    const response = await fetch("/api/artifacts/upload", {
      method: "POST",
      credentials: "include",
      body: formData,
    });
    return parseResponse<UserArtifact>(response);
  },
  getPage(id: string) {
    return apiFetch<PageDocumentRecord>(`/api/pages/${id}`);
  },
  savePage(id: string, req: PageSaveRequest) {
    return apiFetch<PageDocumentRecord>(`/api/pages/${id}`, {
      method: "PATCH",
      body: JSON.stringify(req),
    });
  },
};

export function toFolderNode(record: FolderRecord): FolderNode {
  return {
    id: record.id,
    parentId: record.parentId,
    title: record.title,
  };
}

export function toBreadcrumbItems(records: BreadcrumbRecord[]): BreadcrumbItem[] {
  return records.map((record) => ({ id: record.id, label: record.label }));
}
