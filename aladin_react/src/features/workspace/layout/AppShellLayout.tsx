import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "@/shared/api/client";
import {
  sessionQueryKey,
  useSessionQuery,
} from "@/features/auth/use-session-query";
import { WorkspaceShellProvider } from "@/features/workspace/workspace-state";
import { Sidebar } from "@/features/workspace/sidebar/Sidebar";

function routeDestination(pathname: string) {
  if (pathname.startsWith("/folders")) return "folders";
  if (pathname.startsWith("/sources")) return "sources";
  if (pathname.startsWith("/signals")) return "signals";
  if (pathname.startsWith("/graph")) return "graph";
  return "home";
}

export function AppShellLayout() {
  const sessionQuery = useSessionQuery();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const logoutMutation = useMutation({
    mutationFn: () => api.logout(),
    onSuccess: async () => {
      queryClient.setQueryData(sessionQueryKey, null);
      await queryClient.invalidateQueries({ queryKey: sessionQueryKey });
      navigate("/login", { replace: true });
    },
  });

  return (
    <WorkspaceShellProvider>
      <div className="flex h-screen bg-white text-black">
        <Sidebar
          selectedDestination={routeDestination(location.pathname)}
          userEmail={sessionQuery.data?.email ?? "signed in"}
          onNavigate={navigate}
          onLogout={() => logoutMutation.mutate()}
          logoutPending={logoutMutation.isPending}
        />
        <div className="w-px bg-gray-300" />
        <main className="flex min-w-0 flex-1 overflow-hidden">
          <Outlet />
        </main>
      </div>
    </WorkspaceShellProvider>
  );
}
