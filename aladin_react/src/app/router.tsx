import {
  Navigate,
  createBrowserRouter,
} from "react-router-dom";
import { AuthScreen, ProtectedLayout } from "@/modules/auth/screens";
import {
  WorkspacePlaceholderScreen,
  WorkspaceRouteScreen,
  WorkspaceShellLayoutScreen,
} from "@/modules/workspace/screens";
import { SourcesRouteScreen } from "@/modules/sources/screens";

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <AuthScreen mode="login" />,
  },
  {
    path: "/register",
    element: <AuthScreen mode="register" />,
  },
  {
    element: <ProtectedLayout />,
    children: [
      {
        element: <WorkspaceShellLayoutScreen />,
        children: [
          { index: true, element: <Navigate to="/home" replace /> },
          { path: "/home", element: <WorkspaceRouteScreen destination="home" /> },
          { path: "/folders", element: <WorkspaceRouteScreen destination="folders" /> },
          { path: "/sources", element: <SourcesRouteScreen /> },
          {
            path: "/signals",
            element: (
              <WorkspacePlaceholderScreen
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
              <WorkspacePlaceholderScreen
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
