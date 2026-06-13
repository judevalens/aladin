import type { Subscription } from "rxjs";
import type { AuthSessionService } from "@/services/auth/auth-session-service";
import type { ApiRuntimeConfig } from "@/shared/api/client";
import type { LocalReposAdmin } from "@/repos/local-repos-admin";
import type { LocalSyncRepo } from "@/repos/sync/local-sync-repo";
import type { DesktopSessionStore } from "@/shared/runtime/desktop-session-store";

export interface RealtimeBootDeps {
  authSession: AuthSessionService;
  admin: LocalReposAdmin;
  localSync: LocalSyncRepo;
  desktopSession: DesktopSessionStore;
  config: ApiRuntimeConfig;
  // The server-push websocket pipeline (app events). Started when authenticated,
  // stopped when anonymous. Optional so tests/headless can omit it.
  appEventSource?: { start(): void; stop(): void };
}

export interface RealtimeBoot {
  start(): void;
  stop(): void;
}

export function createRealtimeBoot(deps: RealtimeBootDeps): RealtimeBoot {
  let sessionSub: Subscription | null = null;
  let currentUserId: string | null = null;
  let eventsStarted = false;

  return {
    start() {
      if (sessionSub) return;
      sessionSub = deps.authSession.session().subscribe((snapshot) => {
        if (snapshot.status === "authenticated") {
          const userId = snapshot.user?.id ?? null;
          if (userId !== currentUserId) {
            currentUserId = userId;
          }
          void deps.localSync.setSession({
            apiBaseUrl: deps.config.apiBaseUrl,
            token: deps.desktopSession.getToken(),
          });
          void deps.localSync.drainOutbox().catch(() => undefined);
          // Server-push app events (build-status, …) — start once authenticated.
          if (!eventsStarted) {
            deps.appEventSource?.start();
            eventsStarted = true;
          }
        } else if (snapshot.status === "anonymous") {
          currentUserId = null;
          void deps.localSync.setSession(null);
          void deps.admin.clearWorkspace().catch(() => undefined);
          if (eventsStarted) {
            deps.appEventSource?.stop();
            eventsStarted = false;
          }
        }
      });
    },
    stop() {
      sessionSub?.unsubscribe();
      sessionSub = null;
      if (eventsStarted) {
        deps.appEventSource?.stop();
        eventsStarted = false;
      }
      void deps.localSync.setSession(null);
    },
  };
}
