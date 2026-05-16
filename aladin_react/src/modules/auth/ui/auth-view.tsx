import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface AuthViewProps {
  mode: "login" | "register";
  email: string;
  password: string;
  errorMessage: string | null;
  pending: boolean;
  onEmailChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onSubmit: () => void;
}

export function AuthView({
  mode,
  email,
  password,
  errorMessage,
  pending,
  onEmailChange,
  onPasswordChange,
  onSubmit,
}: AuthViewProps) {
  const isLogin = mode === "login";

  return (
    <div className="app-canvas min-h-screen">
      <div className="grid min-h-screen lg:grid-cols-[1.1fr_0.9fr]">
        <section className="flex min-h-[320px] flex-col justify-between overflow-hidden border-b border-[#e7e5e4] bg-[#fafaf9] px-10 py-12 sm:px-14 sm:py-16 lg:min-h-full lg:border-b-0 lg:border-r">
          <div className="space-y-10">
            <div className="flex items-center gap-2.5">
              <div className="flex h-7 w-7 items-center justify-center rounded-md bg-[#18181b] text-[12px] font-semibold text-[#fafaf9]">
                A
              </div>
              <span className="text-[13px] font-semibold tracking-[-0.01em] text-[#0a0a0a]">Aladin</span>
            </div>

            <div className="space-y-5">
              <h1 className="max-w-xl text-[2.25rem] font-semibold leading-[1.08] tracking-[-0.03em] text-[#0a0a0a] sm:text-[2.625rem]">
                Search, shape, and keep what matters.
              </h1>
              <p className="max-w-lg text-[14px] leading-[1.6] text-[#57534e]">
                A calmer place to collect documents, signals, notes, and connected context — without losing retrieval speed.
              </p>
            </div>

            <div className="grid gap-5 sm:grid-cols-3">
              {[
                ["Structured search", "Ranked streams, filters, and document context."],
                ["Connected notes", "Pages, links, and voice capture in one memory."],
                ["Local control", "Workspace access and agent tooling stay visible."],
              ].map(([title, body]) => (
                <div key={title} className="space-y-1.5 border-t border-[#e7e5e4] pt-3">
                  <div className="text-[12.5px] font-semibold tracking-[-0.005em] text-[#0a0a0a]">{title}</div>
                  <p className="text-[12px] leading-[1.5] text-[#78716c]">{body}</p>
                </div>
              ))}
            </div>
          </div>

          <p className="mt-10 max-w-lg text-[12px] leading-[1.55] text-[#a8a29e]">
            Use the same account you already use for your local workspace.
          </p>
        </section>

        <section className="flex items-center justify-center bg-white px-7 py-12 sm:px-10">
          <div className="w-full max-w-sm">
            <div className="space-y-2">
              <h2 className="text-[1.5rem] font-semibold leading-tight tracking-[-0.02em] text-[#0a0a0a]">
                {isLogin ? "Welcome back" : "Create your account"}
              </h2>
              <p className="text-[13px] leading-[1.55] text-[#78716c]">
                {isLogin
                  ? "Sign in to continue."
                  : "Open a new workspace session."}
              </p>
            </div>
            <form
              className="mt-7 space-y-4"
              onSubmit={(event) => {
                event.preventDefault();
                onSubmit();
              }}
            >
              <div className="space-y-1.5">
                <label className="text-[12px] font-medium text-[#44403c]">Email</label>
                <Input value={email} onChange={(event) => onEmailChange(event.target.value)} type="email" required />
              </div>
              <div className="space-y-1.5">
                <label className="text-[12px] font-medium text-[#44403c]">Password</label>
                <Input
                  value={password}
                  onChange={(event) => onPasswordChange(event.target.value)}
                  type="password"
                  required
                />
              </div>
              {errorMessage ? (
                <div className="rounded-md border border-[#fecaca] bg-[#fef2f2] px-3 py-2 text-[12.5px] leading-[1.5] text-[#991b1b]">
                  {errorMessage}
                </div>
              ) : null}
              <Button className="w-full" disabled={pending}>
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
            <div className="mt-5 text-[12.5px] text-[#78716c]">
              {isLogin ? "Need an account?" : "Already have an account?"}{" "}
              <Link
                className="font-medium text-[#2563eb] hover:underline underline-offset-4"
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
