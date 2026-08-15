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
import { PageEditorEmbed } from "@/modules/pages/editor/page-editor-embed";
import { EntityContextSpike } from "@/modules/entities/ui/entity-context-spike";
import { EntityContextRoute } from "@/modules/entities/ui/entity-context-route";
import { EntitiesIndexUI } from "@/modules/entities/ui/entities-index-ui";
import { EntitiesIndexSpike } from "@/modules/entities/ui/entities-index-spike";
import { EntitiesInboxSpike } from "@/modules/entities/ui/entities-inbox-spike";
import { EntitiesHomeSpike } from "@/modules/entities/ui/entities-home-spike";
import { TutorSpike } from "@/modules/tutor/ui/tutor-spike";
import { TutorReadSpike } from "@/modules/tutor/ui/tutor-read-spike";
import { TutorNotebookSpike } from "@/modules/tutor/ui/tutor-notebook-spike";
import { TutorKindsSpike } from "@/modules/tutor/ui/tutor-kinds-spike";
import { TutorPurposeSpike } from "@/modules/tutor/ui/tutor-purpose-spike";
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
    // The real editor, mounted for a native host to embed in a web view — this is
    // the iPad companion's note editor. Outside the auth shell on purpose: a web
    // view has no session, and the editor authenticates to collab with a token the
    // host injects as window.__ALADIN_EMBED__ (never a query param).
    path: "/embed/page/:id",
    element: <PageEditorEmbed />,
  },
  {
    // Dev-only Tutor spike — the learning-copilot surface on mock data (design/TUTOR_PRD.md
    // rev 3), standalone, no auth. Three scenarios: plan it · read+ask · an aid built.
    path: "/spike/tutor",
    element: <TutorSpike />,
  },
  {
    // Dev-only Tutor spike, take 2 — reading-first, no panes: every affordance is summoned
    // from the text (select → ask) and lands inline. Contrast with /spike/tutor.
    path: "/spike/tutor-read",
    element: <TutorReadSpike />,
  },
  {
    // Dev-only Tutor spike, take 3 — a notebook: a calm single-column index of what a
    // source accrued, modelled on the research Overview. No panes, no chat, no reader.
    path: "/spike/tutor-notebook",
    element: <TutorNotebookSpike />,
  },
  {
    // Dev-only Tutor spike, take 5 — the folder is only a grouper; the product is new
    // ARTIFACT KINDS opening as tabs (canvas, study table). No Tutor surface at all.
    path: "/spike/tutor-kinds",
    element: <TutorKindsSpike />,
  },
  {
    // Dev-only spike — strategy vs learning as a `purpose` DISCRIMINATOR on the existing
    // research folder, not a fourth tree kind. One tree, one Overview component.
    path: "/spike/tutor-purpose",
    element: <TutorPurposeSpike />,
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
