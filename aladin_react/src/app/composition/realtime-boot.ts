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
}

export interface RealtimeBoot {
  start(): void;
  stop(): void;
}

export function createRealtimeBoot(deps: RealtimeBootDeps): RealtimeBoot {
  let sessionSub: Subscription | null = null;
  let currentUserId: string | null = null;

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
        } else if (snapshot.status === "anonymous") {
          currentUserId = null;
          void deps.localSync.setSession(null);
          void deps.admin.clearWorkspace().catch(() => undefined);
        }
      });
    },
    stop() {
      sessionSub?.unsubscribe();
      sessionSub = null;
      void deps.localSync.setSession(null);
    },
  };
}
