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
    element: <ProtectedLayout />,
    children: [
      {
        element: <WorkspaceShellUI />,
        children: [
          { index: true, element: <Navigate to="/home" replace /> },
          { path: "/home", element: <WorkspaceRoute /> },
          { path: "/folders", element: <WorkspaceRoute /> },
          { path: "/sources", element: <SourcesRoute /> },
          {
            path: "/signals",
            element: (
              <PlaceholderDestinationUI
                paneTitle="Signals"
                paneBody="Signals will live here."
                workTitle="Signals"
                workBody="Signal triage is not wired yet."
              />
            ),
          },
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
