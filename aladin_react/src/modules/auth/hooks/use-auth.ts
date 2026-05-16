import { useEffect, useMemo, useState } from "react";
import { NavigateFunction, useLocation, useNavigate } from "react-router-dom";
import { useAppComposition } from "@/app/composition/app-composition";
import { useObservableState } from "@/shared/flow/use-observable-state";

export function useSession() {
  const { services } = useAppComposition();

  useEffect(() => {
    void services.auth.session.ensureBootstrapped();
  }, [services.auth.session]);

  const session = useObservableState(
    () => services.auth.session.observe(),
    () => services.auth.session.getSnapshot(),
    [services.auth.session],
  );

  return {
    isLoading: session.status === "booting",
    data: session.user,
    status: session.status,
  };
}

export function useShellSession(navigate: NavigateFunction) {
  const { services } = useAppComposition();
  const session = useSession();
  const [logoutPending, setLogoutPending] = useState(false);

  return {
    userEmail: session.data?.email ?? "signed in",
    logoutPending,
    onLogout: async () => {
      try {
        setLogoutPending(true);
        await services.auth.session.logout();
        navigate("/login", { replace: true });
      } finally {
        setLogoutPending(false);
      }
    },
  };
}

export function useAuthScreen(mode: "login" | "register") {
  const { services } = useAppComposition();
  const navigate = useNavigate();
  const location = useLocation();
  const session = useSession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const redirectPath = useMemo(() => {
    const next = (location.state as { from?: string } | null)?.from;
    return services.auth.resolveRedirectPath(next);
  }, [location.state, services.auth]);

  return {
    mode,
    sessionReady: !session.isLoading,
    sessionUser: session.data ?? null,
    email,
    password,
    errorMessage,
    pending,
    setEmail,
    setPassword,
    submit: async () => {
      try {
        setPending(true);
        setErrorMessage(null);
        if (mode === "login") {
          await services.auth.session.login({ email, password });
        } else {
          await services.auth.session.register({ email, password });
        }
        navigate(redirectPath, { replace: true });
      } catch (error) {
        setErrorMessage(error instanceof Error ? error.message : "Authentication failed.");
      } finally {
        setPending(false);
      }
    },
  };
}
