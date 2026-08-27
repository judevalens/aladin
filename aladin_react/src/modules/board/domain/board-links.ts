import type { LinkProps } from "../shapes/shape-types";

/**
 * Link-object rules, pure so they're testable without an editor: what counts as a pasted
 * URL, how an unfurl response lands on the shape, and how tall the object should be for
 * what it now shows. The fetch itself lives on BoardContentSource (the host's API plane).
 */

/** What POST /api/unfurl returns (and what the spike fakes). */
export interface UnfurlResult {
  url: string;
  domain: string;
  title: string;
  description: string;
  siteName: string;
  imageUrl: string;
  faviconUrl: string;
}

/**
 * A lone URL pasted as text — the signal that the clipboard held a link, not prose.
 * One token, http(s) or a bare domain-with-path; anything with spaces is prose.
 */
export function pastedUrl(text: string): string | null {
  const trimmed = text.trim();
  if (!trimmed || /\s/.test(trimmed)) return null;
  const candidate = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
  try {
    const parsed = new URL(candidate);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return null;
    // A bare word ("hello") parses as a host — require a dot so prose stays prose.
    if (!parsed.hostname.includes(".")) return null;
    return candidate;
  } catch {
    return null;
  }
}

/** The host to show while the unfurl is in flight (and forever, when it fails). */
export function linkDomain(url: string): string {
  try {
    return new URL(url).hostname.toLowerCase().replace(/^www\./, "");
  } catch {
    return url;
  }
}

/** Object heights per state — image links grow to hold the preview. */
export const LINK_HEIGHT_BARE = 128;
export const LINK_HEIGHT_IMAGE = 252;

/** The prop patch for a resolved unfurl. Height only ever grows the pending rect. */
export function unfurlPatch(result: UnfurlResult): Partial<LinkProps> {
  return {
    status: "ready",
    url: result.url,
    title: result.title,
    description: result.description,
    domain: result.domain,
    image: result.imageUrl,
    favicon: result.faviconUrl,
    h: result.imageUrl ? LINK_HEIGHT_IMAGE : LINK_HEIGHT_BARE,
  };
}

/** The prop patch when the unfurl fails — the bare URL is still a usable link. */
export function unfurlFailedPatch(url: string): Partial<LinkProps> {
  return { status: "failed", domain: linkDomain(url) };
}
