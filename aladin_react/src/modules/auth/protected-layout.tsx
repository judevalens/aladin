import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useSession } from "@/modules/auth/hooks/use-auth-state";

export function ProtectedLayout() {
  const location = useLocation();
  const sessionQuery = useSession();

  if (sessionQuery.isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg text-sm text-ink-2">
        Loading workspace…
      </div>
    );
  }

  if (!sessionQuery.data) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return <Outlet />;
}
