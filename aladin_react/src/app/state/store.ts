import { create } from "zustand";
import { createSessionSlice } from "@/app/state/session-slice";
import type { SessionSlice } from "@/app/state/session-slice";
import { createThemeSlice } from "@/app/state/theme-slice";
import type { ThemeSlice } from "@/app/state/theme-slice";
import { createWorkspaceSlice } from "@/app/state/workspace-slice";
import type { WorkspaceSlice } from "@/app/state/workspace-slice";
import { createShardBuildSlice } from "@/app/state/shard-build-slice";
import type { ShardBuildSlice } from "@/app/state/shard-build-slice";
import { createSignalsSlice } from "@/app/state/signals-slice";
import type { SignalsSlice } from "@/app/state/signals-slice";
import { createTerminalSlice } from "@/app/state/terminal-slice";
import type { TerminalSlice } from "@/app/state/terminal-slice";
import { createMarketSlice } from "@/app/state/market-slice";
import type { MarketSlice } from "@/app/state/market-slice";
import { createCopilotSlice } from "@/app/state/copilot-slice";
import type { CopilotSlice } from "@/app/state/copilot-slice";
import { createNotificationsSlice } from "@/app/state/notifications-slice";
import type { NotificationsSlice } from "@/app/state/notifications-slice";
import { createWatchlistsSlice } from "@/app/state/watchlists-slice";
import type { WatchlistsSlice } from "@/app/state/watchlists-slice";

export type AppStoreState = SessionSlice &
  WorkspaceSlice &
  ThemeSlice &
  ShardBuildSlice &
  SignalsSlice &
  TerminalSlice &
  MarketSlice &
  CopilotSlice &
  NotificationsSlice &
  WatchlistsSlice;

export const useAppStore = create<AppStoreState>()((...args) => ({
  ...createSessionSlice(...args),
  ...createWorkspaceSlice(...args),
  ...createThemeSlice(...args),
  ...createShardBuildSlice(...args),
  ...createSignalsSlice(...args),
  ...createTerminalSlice(...args),
  ...createMarketSlice(...args),
  ...createCopilotSlice(...args),
  ...createNotificationsSlice(...args),
  ...createWatchlistsSlice(...args),
}));
