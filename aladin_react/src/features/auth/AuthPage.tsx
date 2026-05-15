import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { api, ApiError } from "@/shared/api/client";
import { sessionQueryKey, useSessionQuery } from "@/features/auth/use-session-query";
import { useMemo, useState } from "react";

interface AuthPageProps {
  mode: "login" | "register";
}

export function AuthPage({ mode }: AuthPageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const sessionQuery = useSessionQuery();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const redirectPath = useMemo(() => {
    const next = (location.state as { from?: string } | null)?.from;
    return next && next !== "/login" && next !== "/register" ? next : "/home";
  }, [location.state]);

  const authMutation = useMutation({
    mutationFn: async () => {
      setErrorMessage(null);
      if (mode === "login") {
        return api.login({ email, password });
      }
      return api.register({ email, password });
    },
    onSuccess: async (response) => {
      queryClient.setQueryData(sessionQueryKey, response.user);
      await queryClient.invalidateQueries({ queryKey: sessionQueryKey });
      navigate(redirectPath, { replace: true });
    },
    onError: (error) => {
      setErrorMessage(
        error instanceof ApiError ? error.message : "Authentication failed.",
      );
    },
  });

  if (sessionQuery.data) {
    return <Navigate to="/home" replace />;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-white p-6">
      <Card className="w-full max-w-md border-gray-300 bg-white">
        <CardHeader className="space-y-3">
          <div className="inline-flex h-10 w-10 items-center justify-center border border-gray-300 bg-white text-lg font-semibold text-black">
            A
          </div>
          <div className="space-y-1">
            <CardTitle className="text-2xl font-semibold text-black">
              {mode === "login" ? "Welcome back" : "Create workspace access"}
            </CardTitle>
            <CardDescription className="text-sm leading-6 text-gray-700">
              Sign in with your existing workspace account.
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              authMutation.mutate();
            }}
          >
            <div className="space-y-2">
              <label className="text-xs text-gray-500">
                Email
              </label>
              <Input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
            </div>
            <div className="space-y-2">
              <label className="text-xs text-gray-500">
                Password
              </label>
              <Input
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                type="password"
                required
              />
            </div>
            {errorMessage ? (
              <div className="border border-gray-300 bg-white px-3 py-2 text-sm text-black">
                {errorMessage}
              </div>
            ) : null}
            <Button className="w-full" disabled={authMutation.isPending}>
              {authMutation.isPending ? (
                <>
                  <span className="mr-2 h-4 w-4 animate-spin rounded-full border-[1.5px] border-current border-r-transparent" />
                  {mode === "login" ? "Signing in" : "Creating account"}
                </>
              ) : mode === "login" ? (
                "Sign in"
              ) : (
                "Create account"
              )}
            </Button>
          </form>
          <div className="mt-5 text-sm text-gray-700">
            {mode === "login" ? "Need an account?" : "Already have an account?"}{" "}
            <Link
              className="font-medium text-black underline underline-offset-4"
              to={mode === "login" ? "/register" : "/login"}
            >
              {mode === "login" ? "Register" : "Sign in"}
            </Link>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
