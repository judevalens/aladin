import { create } from "zustand";
import { createSessionSlice } from "@/app/state/session-slice";
import type { SessionSlice } from "@/app/state/session-slice";
import { createThemeSlice } from "@/app/state/theme-slice";
import type { ThemeSlice } from "@/app/state/theme-slice";
import { createWorkspaceSlice } from "@/app/state/workspace-slice";
import type { WorkspaceSlice } from "@/app/state/workspace-slice";
import { createShardBuildSlice } from "@/app/state/shard-build-slice";
import type { ShardBuildSlice } from "@/app/state/shard-build-slice";

export type AppStoreState = SessionSlice & WorkspaceSlice & ThemeSlice & ShardBuildSlice;

export const useAppStore = create<AppStoreState>()((...args) => ({
  ...createSessionSlice(...args),
  ...createWorkspaceSlice(...args),
  ...createThemeSlice(...args),
  ...createShardBuildSlice(...args),
}));
