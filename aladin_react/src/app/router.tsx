import { Navigate, createBrowserRouter } from "react-router-dom";
import { ProtectedLayout } from "@/modules/auth/protected-layout";
import { AuthUI } from "@/modules/auth/ui/auth-ui";
import { BrowserPaneUI } from "@/modules/workspace/ui/browser-pane-ui";
import { RenameDialogUI } from "@/modules/workspace/ui/rename-dialog-ui";
import { VoiceCaptureDialogUI } from "@/modules/workspace/ui/voice-capture-dialog-ui";
import { WorkPaneUI } from "@/modules/workspace/ui/work-pane-ui";
import {
  PlaceholderDestinationUI,
  WorkspaceShellUI,
} from "@/modules/workspace/ui/workspace-shell-ui";
import { SourcesRoute } from "@/modules/sources/sources-route";
import { InsightsUI } from "@/modules/insights/ui/insights-ui";
import { SandboxSpike } from "@/modules/doc-surface/spike/sandbox-spike";
import { EditorPreview } from "@/modules/pages/editor/editor-preview";
import { EntityContextSpike } from "@/modules/entities/ui/entity-context-spike";
import { EntityContextRoute } from "@/modules/entities/ui/entity-context-route";
import { EntitiesIndexUI } from "@/modules/entities/ui/entities-index-ui";
import { EntitiesIndexSpike } from "@/modules/entities/ui/entities-index-spike";
import { EntitiesInboxSpike } from "@/modules/entities/ui/entities-inbox-spike";
import { EntitiesHomeSpike } from "@/modules/entities/ui/entities-home-spike";
import { TickerRoute } from "@/modules/tickers/ui/ticker-route";
import { MarketsUI } from "@/modules/markets/ui/markets-ui";
import { TickerModal } from "@/modules/markets/ui/ticker-modal";

function WorkspaceRoute() {
  return (
    <>
      <BrowserPaneUI />
      <WorkPaneUI />
      <RenameDialogUI />
      <VoiceCaptureDialogUI />
    </>
  );
}

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <AuthUI mode="login" />,
  },
  {
    path: "/register",
    element: <AuthUI mode="register" />,
  },
  {
    // Phase 0 Doc Surface spike — standalone, no auth, for browser + tauri:dev.
    path: "/spike/sandbox",
    element: <SandboxSpike />,
  },
  {
    // Dev-only Markets spike — the trading surface on placeholder data, no auth.
    path: "/spike/markets",
    element: (
      <div className="flex h-screen overflow-hidden">
        <MarketsUI />
        <TickerModal />
      </div>
    ),
  },
  {
    // Dev-only editor preview — standalone, no auth/collab, for iterating on
    // the block editor's look-and-feel outside the login wall.
    path: "/spike/editor",
    element: <EditorPreview />,
  },
  {
    // Dev-only Entity Context spike — Phase A of the entity surface on mock
    // data (design/ENTITY_CONTEXT_PRD.md), standalone, no auth.
    path: "/spike/entity-context",
    element: <EntityContextSpike />,
  },
  {
    // Dev-only Entities index spike — the masonry cards on mock data, no auth.
    path: "/spike/entities-index",
    element: <EntitiesIndexSpike />,
  },
  {
    // Dev-only Entities inbox spike — the default triage landing on mock data, no auth.
    path: "/spike/entities-inbox",
    element: <EntitiesInboxSpike />,
  },
  {
    // Dev-only Entities HOME spike — the constellation/map reimagining on mock data.
    path: "/spike/entities-home",
    element: <EntitiesHomeSpike />,
  },
  {
    element: <ProtectedLayout />,
    children: [
      {
        element: <WorkspaceShellUI />,
        children: [
          { index: true, element: <Navigate to="/home" replace /> },
          { path: "/home", element: <WorkspaceRoute /> },
          { path: "/markets", element: <MarketsUI /> },
          { path: "/folders", element: <WorkspaceRoute /> },
          { path: "/sources", element: <SourcesRoute /> },
          { path: "/insights", element: <InsightsUI /> },
          { path: "/entities", element: <EntitiesIndexUI /> },
          // The Entity Context surface. Router history is the back-trail, so an edge
          // click ("pull the thread") is undone by the normal back gesture.
          { path: "/entity/:entityId", element: <EntityContextRoute /> },
          // Ticker detail — the landing for a security selected from the command box.
          { path: "/ticker/:symbol", element: <TickerRoute /> },
          {
            path: "/graph",
            element: (
              <PlaceholderDestinationUI
                paneTitle="Graph"
                paneBody="Graph will remain a workspace-wide context view."
                workTitle="Graph"
                workBody="Workspace-wide graph exploration is not wired yet."
              />
            ),
          },
        ],
      },
    ],
  },
]);
