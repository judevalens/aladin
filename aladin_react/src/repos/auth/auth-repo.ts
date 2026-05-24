import type { ApiClient } from "@/shared/api/client";
import type { AuthRequest, AuthResponse } from "@/shared/api/models";

export interface AuthRepo {
  me(): Promise<AuthResponse>;
  login(input: AuthRequest): Promise<AuthResponse>;
  register(input: AuthRequest): Promise<AuthResponse>;
  logout(): Promise<void>;
}

export function createAuthRepo(client: ApiClient): AuthRepo {
  return {
    me: () => client.fetch<AuthResponse>("/api/auth/me"),
    login: (input) =>
      client.fetch<AuthResponse>("/api/auth/desktop/login", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    register: (input) =>
      client.fetch<AuthResponse>("/api/auth/desktop/register", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    logout: () => client.fetch<void>("/api/auth/logout", { method: "POST" }),
  };
}
