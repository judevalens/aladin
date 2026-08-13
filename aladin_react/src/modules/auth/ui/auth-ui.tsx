import { Link, Navigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuthState } from "@/modules/auth/hooks/use-auth-state";

export function AuthUI({ mode }: { mode: "login" | "register" }) {
  const state = useAuthState(mode);

  if (state.sessionReady && state.sessionUser) {
    return <Navigate to="/home" replace />;
  }

  const {
    email,
    password,
    errorMessage,
    pending,
    setEmail,
    setPassword,
    submit,
  } = state;
  const isLogin = mode === "login";

  return (
    <div className="min-h-screen bg-bg font-sans text-ink">
      <div className="grid min-h-screen lg:grid-cols-[1.1fr_0.9fr]">
        <section className="flex min-h-[320px] flex-col justify-between overflow-hidden border-b border-line bg-panel px-10 py-12 sm:px-14 sm:py-16 lg:min-h-full lg:border-b-0 lg:border-r">
          <div className="space-y-10">
            <div className="flex items-center gap-2.5">
              <div className="grid size-7 place-items-center rounded-control bg-amber font-display text-small font-bold text-primary-foreground">
                A
              </div>
              <span className="text-body font-semibold tracking-[-0.01em] text-ink">Aladin</span>
            </div>

            <div className="space-y-5">
              <h1 className="max-w-xl font-display text-display font-semibold tracking-[-0.03em] text-ink">
                Search, shape, and keep what matters.
              </h1>
              <p className="max-w-lg text-lead text-ink-2">
                A calmer place to collect documents, signals, notes, and connected context — without losing retrieval speed.
              </p>
            </div>

            <div className="grid gap-5 sm:grid-cols-3">
              {[
                ["Structured search", "Ranked streams, filters, and document context."],
                ["Connected notes", "Pages, links, and voice capture in one memory."],
                ["Local control", "Workspace access and agent tooling stay visible."],
              ].map(([title, body]) => (
                <div key={title} className="space-y-1.5 border-t border-line pt-3">
                  <div className="text-small font-semibold tracking-[-0.005em] text-ink">{title}</div>
                  <p className="text-small text-ink-3">{body}</p>
                </div>
              ))}
            </div>
          </div>

          <p className="mt-10 max-w-lg text-small text-ink-4">
            Use the same account you already use for your local workspace.
          </p>
        </section>

        <section className="flex items-center justify-center bg-bg px-7 py-12 sm:px-10">
          <div className="w-full max-w-sm">
            <div className="space-y-2">
              <h2 className="font-display text-title font-semibold tracking-[-0.02em] text-ink">
                {isLogin ? "Welcome back" : "Create your account"}
              </h2>
              <p className="text-body text-ink-3">
                {isLogin
                  ? "Sign in to continue."
                  : "Open a new workspace session."}
              </p>
            </div>
            <form
              className="mt-7 space-y-4"
              onSubmit={(event) => {
                event.preventDefault();
                submit();
              }}
            >
              <div className="space-y-1.5">
                <label className="text-small font-medium text-ink-2">Email</label>
                <Input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
              </div>
              <div className="space-y-1.5">
                <label className="text-small font-medium text-ink-2">Password</label>
                <Input
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  type="password"
                  required
                />
              </div>
              {errorMessage ? (
                <div className="rounded-control border border-against/40 bg-against/10 px-3 py-2 text-small text-against">
                  {errorMessage}
                </div>
              ) : null}
              <Button type="submit" className="w-full" disabled={pending}>
                {pending ? (
                  <>
                    <span className="mr-2 h-3.5 w-3.5 animate-spin rounded-full border-[1.5px] border-current border-r-transparent" />
                    {isLogin ? "Signing in" : "Creating account"}
                  </>
                ) : isLogin ? (
                  "Sign in"
                ) : (
                  "Create account"
                )}
              </Button>
            </form>
            <div className="mt-5 text-small text-ink-3">
              {isLogin ? "Need an account?" : "Already have an account?"}{" "}
              <Link
                className="font-medium text-amber underline-offset-4 hover:underline"
                to={isLogin ? "/register" : "/login"}
              >
                {isLogin ? "Register" : "Sign in"}
              </Link>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
