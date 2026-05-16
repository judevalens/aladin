import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuthScreen, useSession } from "@/modules/auth/hooks/use-auth";
import { AuthView } from "@/modules/auth/ui/auth-view";

export function AuthScreen({ mode }: { mode: "login" | "register" }) {
  const screen = useAuthScreen(mode);

  if (screen.sessionReady && screen.sessionUser) {
    return <Navigate to="/home" replace />;
  }

  return (
    <AuthView
      mode={screen.mode}
      email={screen.email}
      password={screen.password}
      errorMessage={screen.errorMessage}
      pending={screen.pending}
      onEmailChange={screen.setEmail}
      onPasswordChange={screen.setPassword}
      onSubmit={screen.submit}
    />
  );
}

export function ProtectedLayout() {
  const location = useLocation();
  const sessionQuery = useSession();

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
