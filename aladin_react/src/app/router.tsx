import {
  Navigate,
  Outlet,
  createBrowserRouter,
  useLocation,
} from "react-router-dom";
import { AuthPage } from "@/features/auth/AuthPage";
import { useSessionQuery } from "@/features/auth/use-session-query";
import { AppShellLayout } from "@/features/workspace/layout/AppShellLayout";
import { HomeWorkspaceRoute } from "@/features/workspace/routes/HomeWorkspaceRoute";
import { SourcesRoute } from "@/features/sources/SourcesRoute";
import { PlaceholderDestinationRoute } from "@/features/workspace/routes/PlaceholderDestinationRoute";

function ProtectedLayout() {
  const location = useLocation();
  const sessionQuery = useSessionQuery();

  if (sessionQuery.isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-white text-sm text-gray-700">
        Loading workspace…
      </div>
    );
  }

  if (!sessionQuery.data) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return <Outlet />;
}

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <AuthPage mode="login" />,
  },
  {
    path: "/register",
    element: <AuthPage mode="register" />,
  },
  {
    element: <ProtectedLayout />,
    children: [
      {
        element: <AppShellLayout />,
        children: [
          { index: true, element: <Navigate to="/home" replace /> },
          { path: "/home", element: <HomeWorkspaceRoute destination="home" /> },
          { path: "/folders", element: <HomeWorkspaceRoute destination="folders" /> },
          { path: "/sources", element: <SourcesRoute /> },
          {
            path: "/signals",
            element: (
              <PlaceholderDestinationRoute
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
              <PlaceholderDestinationRoute
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
