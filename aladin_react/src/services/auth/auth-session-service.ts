import { BehaviorSubject, type Observable } from "rxjs";
import type { AuthRepo } from "@/repos/auth/auth-repo";
import type { DesktopSessionStore } from "@/shared/runtime/desktop-session-store";
import type { AuthUser } from "@/shared/api/models";

export interface AuthSessionSnapshot {
  status: "booting" | "anonymous" | "authenticated";
  user: AuthUser | null;
}

const bootingSnapshot: AuthSessionSnapshot = {
  status: "booting",
  user: null,
};

export class AuthSessionService {
  private readonly subject = new BehaviorSubject<AuthSessionSnapshot>(bootingSnapshot);
  private bootstrapPromise: Promise<void> | null = null;

  constructor(
    private readonly repo: AuthRepo,
    private readonly desktopSessionStore: DesktopSessionStore,
  ) {}

  session(): Observable<AuthSessionSnapshot> {
    this.bootstrapIfNeeded();
    return this.subject;
  }

  async login(input: { email: string; password: string }) {
    const response = await this.repo.login(input);
    if (response.token) {
      this.desktopSessionStore.save({
        token: response.token,
        expiresAt: response.expiresAt ?? null,
      });
    }
    this.subject.next({
      status: "authenticated",
      user: response.user,
    });
    return response.user;
  }

  async register(input: { email: string; password: string }) {
    const response = await this.repo.register(input);
    if (response.token) {
      this.desktopSessionStore.save({
        token: response.token,
        expiresAt: response.expiresAt ?? null,
      });
    }
    this.subject.next({
      status: "authenticated",
      user: response.user,
    });
    return response.user;
  }

  async logout() {
    try {
      await this.repo.logout();
    } finally {
      this.desktopSessionStore.clear();
      this.subject.next({
        status: "anonymous",
        user: null,
      });
    }
  }

  private async bootstrap() {
    try {
      const response = await this.repo.me();
      this.subject.next({
        status: "authenticated",
        user: response.user,
      });
    } catch {
      this.desktopSessionStore.clear();
      this.subject.next({
        status: "anonymous",
        user: null,
      });
    } finally {
      this.bootstrapPromise = null;
    }
  }

  private bootstrapIfNeeded() {
    if (this.subject.getValue().status !== "booting") {
      return;
    }
    if (!this.bootstrapPromise) {
      this.bootstrapPromise = this.bootstrap();
    }
  }
}
