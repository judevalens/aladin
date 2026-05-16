import type { Source } from "@/shared/api/models";
import { formatHumanDate, formatRelativeTime, titleCase } from "@/shared/lib/utils";

export function isOperational(syncState: string) {
  const state = syncState.toLowerCase();
  return state === "active" || state === "live" || state === "operational" || state === "healthy";
}

export function providerLabel(source: Source) {
  const type = source.type.toLowerCase();
  if (type.includes("bluesky")) return "Bluesky";
  if (type.includes("reddit")) return "Reddit";
  if (type.includes("github")) return "GitHub";
  return titleCase(source.type);
}

export function descriptionLine(source: Source) {
  if (source.type.toLowerCase().includes("bluesky")) {
    const query = typeof source.config.query === "string" ? source.config.query : null;
    return query ? `Tracks Bluesky results for “${query}”.` : "Tracks a Bluesky search stream.";
  }
  return `${providerLabel(source)} stream`;
}

export function healthLabel(source: Source) {
  return isOperational(source.syncState) ? "Healthy" : titleCase(source.syncState);
}

export function lastRefreshSummary(source: Source) {
  return source.lastSyncedAt ? formatRelativeTime(source.lastSyncedAt) : "Awaiting first refresh";
}

export function formatSourceFacts(source: Source) {
  return [
    { label: "Provider", value: providerLabel(source) },
    { label: "Created", value: formatHumanDate(source.createdAt) },
    { label: "Sync mode", value: titleCase(source.syncMode) },
    { label: "Last synced", value: formatRelativeTime(source.lastSyncedAt) },
  ];
}
