import { create } from "zustand";
import type { SessionSlice } from "@/app/state/session-slice";
import { createSessionSlice } from "@/app/state/session-slice";
import type { WorkspaceSlice } from "@/app/state/workspace-slice";
import { createWorkspaceSlice } from "@/app/state/workspace-slice";

export type AppStoreState = SessionSlice & WorkspaceSlice;

export const useAppStore = create<AppStoreState>()((...args) => ({
  ...createSessionSlice(...args),
  ...createWorkspaceSlice(...args),
}));
