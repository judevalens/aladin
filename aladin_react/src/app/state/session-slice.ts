import type { StateCreator } from "zustand";
import type { AuthUser } from "@/shared/api/models";

type SessionStatus = "booting" | "anonymous" | "authenticated";

export interface SessionState {
  status: SessionStatus;
  user: AuthUser | null;
  redirectPath: string | null;
}

export interface SessionSlice {
  session: SessionState;
  setSessionBooting: () => void;
  setSessionAuthenticated: (user: AuthUser) => void;
  setSessionAnonymous: () => void;
  setRedirectPath: (path: string | null) => void;
}

export const initialSessionState: SessionState = {
  status: "booting",
  user: null,
  redirectPath: null,
};

export const createSessionSlice: StateCreator<SessionSlice, [], [], SessionSlice> = (set) => ({
  session: initialSessionState,
  setSessionBooting: () => set(() => ({ session: { ...initialSessionState, status: "booting" } })),
  setSessionAuthenticated: (user) =>
    set((state) => ({
      session: {
        ...state.session,
        status: "authenticated",
        user,
      },
    })),
  setSessionAnonymous: () =>
    set((state) => ({
      session: {
        ...state.session,
        status: "anonymous",
        user: null,
      },
    })),
  setRedirectPath: (path) =>
    set((state) => ({
      session: {
        ...state.session,
        redirectPath: path,
      },
    })),
});
