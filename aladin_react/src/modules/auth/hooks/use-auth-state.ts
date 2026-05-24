import { useMemo, useState } from "react";
import { NavigateFunction, useLocation, useNavigate } from "react-router-dom";
import { useAppComposition } from "@/app/composition/app-composition";
import type { AuthSessionSnapshot } from "@/services/auth/auth-session-service";
import { useObservableState } from "@/shared/flow/use-observable-state";

export interface SessionState {
  isLoading: boolean;
  data: AuthSessionSnapshot["user"] | null;
  status: string;
}

export interface ShellSessionState {
  userEmail: string;
  logoutPending: boolean;
  onLogout: () => Promise<void>;
}

export interface AuthState {
  mode: "login" | "register";
  sessionReady: boolean;
  sessionUser: SessionState["data"];
  email: string;
  password: string;
  errorMessage: string | null;
  pending: boolean;
  setEmail: (value: string) => void;
  setPassword: (value: string) => void;
  submit: () => Promise<void>;
}

export function useSession(): SessionState {
  const { services } = useAppComposition();
  const sessionState = useObservableState(services.auth.session.session());

  if (sessionState.status !== "data") {
    return { isLoading: true, data: null, status: "booting" };
  }

  const snapshot = sessionState.value;
  return {
    isLoading: snapshot.status === "booting",
    data: snapshot.user,
    status: snapshot.status,
  };
}

export function useShellSession(navigate: NavigateFunction): ShellSessionState {
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

export function useAuthState(mode: "login" | "register"): AuthState {
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
