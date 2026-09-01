import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { FlaskConical } from "lucide-react";
import type { Artifact } from "@/shared/api/models";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire, shardBuildKey } from "@/app/state/shard-build-slice";
import type { ShardChannel } from "@/app/state/shard-build-slice";
import { createBridgeHost } from "@/modules/doc-surface/bridge/bridge-host";
import type { BridgeHost } from "@/modules/doc-surface/bridge/bridge-host";
import { createBridgeV2Host } from "../bridge/bridge-v2-host";
import { useShardRelease } from "../hooks/use-shard-release";
import { Button } from "@/components/ui/button";
import { Icon } from "@/components/ui/icon";
import { useShardContentToken } from "@/modules/doc-surface/hooks/use-shard-content-token";
import { ShardAccessNotice } from "@/modules/doc-surface/ui/shard-access-notice";
import { cn } from "@/lib/utils";

// useServedUrl resolves the content-origin URL for a Doc Surface page. In web dev
// apiBaseUrl is "" so this is relative (/content/...) and goes through the vite
// proxy (same origin); in the desktop app it is the absolute API origin.
//
// Both web and desktop use bearer sessions. The iframe URL must carry a separate
// content-only credential, NEVER the session bearer that shard JS could steal.
//
// channel selects the published (default) or draft build; nonce (a build id) is
// appended so a fresh build reloads the iframe by changing its src.
function useServedUrl(pageId: string, channel: ShardChannel, nonce?: string, buildId?: string, enabled = true) {
  const { runtime } = useAppComposition();
  const { token: contentToken, error, retry } = useShardContentToken(
    enabled ? runtime.contentTokens : null,
    JSON.stringify([runtime.config.apiBaseUrl, pageId, channel, nonce, buildId]),
  );

  // Memoized so the iframe reloads ONLY on channel/build/token changes. The
  // theme is deliberately read non-reactively (getState): it stamps data-theme
  // for a correct FIRST paint; live switches ride the bridge push with no
  // reload, and the next natural reload picks up the then-current theme.
  const src = useMemo(() => {
    if (!enabled || !contentToken) return null;
    const base = runtime.config.apiBaseUrl;
    const params = new URLSearchParams();
    if (contentToken) params.set("access_token", contentToken);
    if (channel === "draft") params.set("channel", "draft");
    if (nonce) params.set("v", nonce);
    if (buildId) params.set("build_id", buildId);
    const theme = useAppStore.getState().theme;
    if (theme) params.set("theme", theme);
    // In the desktop app, signal the serve route to emit vendor://deps/<sha> import
    // URLs so deps load from the local Tauri cache (zero network after first fetch).
    if (typeof window !== "undefined" && "__TAURI_INTERNALS__" in window) {
      params.set("client", "tauri");
    }
    const q = params.toString();
    return `${base}/content/${pageId}/${q ? `?${q}` : ""}`;
  }, [runtime, pageId, channel, nonce, buildId, contentToken, enabled]);
  return { src, error, retry };
}

// useShardBuild seeds a channel's build state for a page (one fetch on mount) and
// returns the live entry from the store (updated by realtime build-status events).
function useShardBuild(pageId: string, channel: ShardChannel) {
  const { runtime } = useAppComposition();
  const setShardBuild = useAppStore((s) => s.setShardBuild);
  const build = useAppStore((s) => s.shardBuilds[shardBuildKey(pageId, channel)]);

  useEffect(() => {
    let cancelled = false;
    void runtime.apis.shards
      .getBuildState(pageId, channel)
      .then((wire) => {
        if (!cancelled && wire?.page_id) setShardBuild(shardBuildFromWire(wire));
      })
      .catch(() => undefined); // no build yet / offline — fine, stays unseeded
    return () => {
      cancelled = true;
    };
  }, [pageId, channel, runtime, setShardBuild]);

  return build;
}

function BuildErrorOverlay({ log }: { log: string }) {
  return (
    <div className="absolute inset-0 z-20 overflow-auto bg-bg/95 p-6 backdrop-blur">
      <div className="mb-2 text-small font-medium text-against">Build failed</div>
      <pre className="whitespace-pre-wrap break-words font-mono text-small leading-relaxed text-ink-2">
        {log || "(no diagnostics)"}
      </pre>
    </div>
  );
}

/**
 * DocSurfaceUI renders one agent-authored "app" page in a sandboxed iframe.
 *
 * Isolation: `sandbox="allow-scripts"` with NO `allow-same-origin` → the frame
 * has an opaque origin and cannot reach Aladin's DOM/cookies/storage. The only
 * channel back is the source-window-checked bridge/1 host (bridge-host.ts).
 *
 * Published code and data are the normal view. Draft builds only reload an
 * explicit preview (or an unpublished app); publishing exits that preview.
 */
export function DocSurfaceUI({ artifact, hidden = false, controlsTarget }: { artifact: Artifact; hidden?: boolean; controlsTarget?: HTMLElement | null }) {
  return <ShardSurface key={artifact.id} artifact={artifact} hidden={hidden} controlsTarget={controlsTarget} />;
}

function ShardSurface({ artifact, hidden, controlsTarget }: { artifact: Artifact; hidden: boolean; controlsTarget?: HTMLElement | null }) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const previewDescriptionId = useId();
  const draft = useShardBuild(artifact.id, "draft");
  const publishedBuild = useShardBuild(artifact.id, "published");
  const publication = useAppStore((s) => s.shardPublications[artifact.id]);
  const theme = useAppStore((s) => s.theme);

  // Retain the last usable draft while an author is building the next one.
  const [draftNonce, setDraftNonce] = useState<string>("");
  const [publishedNonce, setPublishedNonce] = useState<string>("");
  useEffect(() => {
    if (draft?.status === "ok" && draft.buildId) setDraftNonce(draft.buildId);
  }, [draft?.status, draft?.buildId]);
  useEffect(() => {
    if (publishedBuild?.status === "ok" && publishedBuild.buildId) setPublishedNonce(publishedBuild.buildId);
  }, [publishedBuild?.status, publishedBuild?.buildId]);

  const draftRelease = useShardRelease(artifact.id, "draft", draftNonce);
  const legacyBuildNonce = draftRelease.available && draftRelease.value?.protocol === "bridge/1" ? publishedNonce : "";
  const publishedRelease = useShardRelease(artifact.id, "published", legacyBuildNonce, publication);
  const [preview, setPreview] = useState<{ publication?: string }>();
  const publicationIdentity = publication?.eventId ?? (publishedRelease.value?.protocol === "bridge/1" ? publishedNonce : undefined);
  // Selecting preview is local to this tab and this publication. A committed
  // publication returns normal use to its published records immediately.
  const explicitPreview = preview !== undefined && preview.publication === publicationIdentity;
  const unpublished = publishedRelease.value !== undefined && !publishedRelease.available;
  const channel: ShardChannel = explicitPreview || unpublished ? "draft" : "published";
  const selected = channel === "draft" ? draftRelease : publishedRelease;
  const release = selected.available ? selected.value : undefined;
  const { runtime } = useAppComposition();
  const nonce = channel === "draft" ? draftNonce : release?.protocol === "bridge/1" ? publishedNonce : undefined;
  const { src, error: accessError, retry } = useServedUrl(artifact.id, channel, nonce, release?.protocol === "bridge/2" ? release.buildId : undefined, !!release);

  const status = draft?.status;
  const showError = channel === "draft" && status === "failed";
  const draftWarning = release?.protocol === "bridge/1" ? "Saved data is shared with published." : "Separate test data; not visible through MCP.";
  const controlLabel = channel === "draft" ? publishedRelease.available ? "Back to published" : "Draft preview" : "Preview draft";
  const controlDescription = channel === "draft"
    ? `${unpublished ? "Unpublished · " : ""}Draft preview. ${draftWarning}${status === "building" ? " Building…" : status === "failed" ? " Build failed." : ""}`
    : "Published. Preview draft to test changes without affecting published data.";
  const control = (
    <span className="inline-flex" title={controlDescription}>
      <Button
        size="icon-xs"
        variant="ghost"
        aria-label={controlLabel}
        aria-pressed={channel === "draft"}
        aria-describedby={previewDescriptionId}
        title={`${controlDescription}${channel === "draft" && publishedRelease.available ? " Back to published." : ""}`}
        disabled={channel === "draft" ? !publishedRelease.available : !draftRelease.available && status !== "failed"}
        onClick={() => setPreview(channel === "draft" ? undefined : { publication: publicationIdentity })}
        className={cn("h-6 w-6 rounded-tap", channel === "draft" ? "bg-amber-soft text-amber hover:text-amber" : "text-ink-3 hover:text-ink", showError && "text-against", channel === "draft" && status === "building" && "animate-pulse")}
      >
        <Icon as={FlaskConical} />
      </Button>
      <span id={previewDescriptionId} className="sr-only">{channel === "draft" ? draftWarning : controlDescription}</span>
    </span>
  );

  // One bridge host per iframe: answers the kit's bridge/1 requests (source-
  // window checked), proxies shard local state, and pushes theme switches.
  const hostRef = useRef<BridgeHost | null>(null);
  useLayoutEffect(() => {
    if (!release) return;
    const host = release.protocol === "bridge/2" ? createBridgeV2Host({
      target: { shardId: artifact.id, environment: channel, contractHash: release.contractHash },
      buildId: release.buildId,
      getWindow: () => iframeRef.current?.contentWindow,
      getTheme: () => useAppStore.getState().theme,
      hub: runtime.apis.shardResources,
    }) : createBridgeHost({
      pageId: artifact.id,
      getWindow: () => iframeRef.current?.contentWindow,
      getTheme: () => useAppStore.getState().theme,
      kv: runtime.apis.shardKV,
      api: runtime.apis.shards,
      hub: runtime.apis.shardDataHub,
    });
    host.attach();
    hostRef.current = host;
    return () => {
      host.detach();
      hostRef.current = null;
    };
  }, [artifact.id, channel, runtime, release, src]);

  // Live theme sync — pushed even while this frame is CSS-hidden in the
  // keep-alive set, so it re-surfaces already in the right theme.
  useEffect(() => {
    hostRef.current?.pushTheme(theme);
  }, [theme]);

  return (
    <div className={cn("relative h-full w-full", hidden && "hidden")}>
      {/* Only the active kept-alive pane owns the top-bar control. Portalling it
          leaves preview state and the iframe lifetime with this shard. */}
      {!hidden && (controlsTarget ? createPortal(control, controlsTarget) : (
        <div className="absolute right-3 top-3 z-30">{control}</div>
      ))}
      {src && release ? (
        <iframe
          ref={iframeRef}
          title={artifact.title}
          src={src}
          sandbox="allow-scripts"
          className="h-full w-full border-0 bg-bg"
        />
      ) : selected.value && !selected.available ? (
        <div role="status" className="flex h-full items-center justify-center px-6 text-small text-ink-3">This shard has not been built yet.</div>
      ) : <ShardAccessNotice error={accessError || selected.error} retry={() => { retry(); selected.retry(); }} />}
      {showError && <BuildErrorOverlay log={draft?.errors ?? ""} />}
    </div>
  );
}

// The shard-only keep-alive that used to live here (DocSurfaceKeepAlive + useKeepAliveIds) is
// gone: the work pane now keeps EVERY tab kind mounted, so shards no longer need their own
// mechanism, and two nested keep-alive windows would have double-mounted the iframes. See
// `modules/workspace/hooks/use-keep-alive.ts` — the `hidden` prop below is still how a shard
// learns it is off screen.
