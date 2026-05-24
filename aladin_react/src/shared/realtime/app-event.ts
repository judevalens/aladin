export interface AppEventEnvelope {
  eventId: string;
  type: string;
  subscriptionKey: {
    stream: string;
    resourceKind: string;
    resourceId: string;
    qualifiers?: Record<string, string>;
  };
  payload: unknown;
  occurredAt: string;
}

export type WorkspaceEventType =
  | "artifact.created"
  | "artifact.updated"
  | "artifact.deleted"
  | "folder.created"
  | "folder.updated"
  | "folder.deleted"
  | "page.updated";

export const KNOWN_WORKSPACE_EVENT_TYPES: ReadonlySet<WorkspaceEventType> = new Set([
  "artifact.created",
  "artifact.updated",
  "artifact.deleted",
  "folder.created",
  "folder.updated",
  "folder.deleted",
  "page.updated",
]);

export function isWorkspaceEventType(type: string): type is WorkspaceEventType {
  return KNOWN_WORKSPACE_EVENT_TYPES.has(type as WorkspaceEventType);
}
