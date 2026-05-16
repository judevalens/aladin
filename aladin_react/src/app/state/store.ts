import { create } from "zustand";
import type { PageSessionSlice } from "@/app/state/page-session-slice";
import { createPageSessionSlice } from "@/app/state/page-session-slice";
import type { SessionSlice } from "@/app/state/session-slice";
import { createSessionSlice } from "@/app/state/session-slice";
import type { SourcesUiSlice } from "@/app/state/sources-ui-slice";
import { createSourcesUiSlice } from "@/app/state/sources-ui-slice";
import type { WorkspaceSlice } from "@/app/state/workspace-slice";
import { createWorkspaceSlice } from "@/app/state/workspace-slice";

export type AppStoreState = SessionSlice & WorkspaceSlice & SourcesUiSlice & PageSessionSlice;

export const useAppStore = create<AppStoreState>()((...args) => ({
  ...createSessionSlice(...args),
  ...createWorkspaceSlice(...args),
  ...createSourcesUiSlice(...args),
  ...createPageSessionSlice(...args),
}));
