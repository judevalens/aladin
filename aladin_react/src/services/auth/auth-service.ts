import type { AuthUser } from "@/shared/api/models";

export function resolveRedirectPath(from?: string | null) {
  return from && from !== "/login" && from !== "/register" ? from : "/home";
}

export function resolveSessionStatus(user: AuthUser | null | undefined) {
  return user ? "authenticated" : "anonymous";
}
