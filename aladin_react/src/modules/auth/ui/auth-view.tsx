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
        <section className="flex min-h-[320px] flex-col justify-between overflow-hidden border-b border-[#e4e4e4] px-7 py-8 sm:px-10 sm:py-10 lg:min-h-full lg:border-b-0 lg:border-r">
          <div className="space-y-8">
            <div className="space-y-4">
              <div className="inline-flex h-11 w-11 items-center justify-center rounded-[12px] border border-[#d9d6cf] bg-white text-lg font-semibold text-[#111111]">
                A
              </div>
              <div className="space-y-3">
                <div className="eyebrow">Knowledge workspace</div>
                <h1 className="max-w-xl text-[2.5rem] font-semibold leading-[1.02] tracking-[-0.06em] text-[#111111] sm:text-[3.5rem]">
                  Search, shape, and keep what matters.
                </h1>
                <p className="section-copy">
                  Aladin gives your workspace a calmer place to collect documents, signals, notes, and connected
                  context without losing retrieval speed.
                </p>
              </div>
            </div>

            <div className="grid gap-4 border-t border-[#e4e4e4] pt-6 sm:grid-cols-3">
              {[
                ["Structured search", "Ranked streams, filters, and document context stay close to the work."],
                ["Connected notes", "Pages, links, and voice capture sit inside the same workspace memory."],
                ["Local control", "Workspace access and agent tooling remain visible and deliberate."],
              ].map(([title, body]) => (
                <div key={title} className="border-l border-[#e4e4e4] pl-4">
                  <div className="text-sm font-semibold tracking-[-0.02em] text-[#111111]">{title}</div>
                  <p className="mt-2 text-sm leading-6 text-[#52525b]">{body}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="mt-8 border-t border-[#e4e4e4] pt-5">
            <div className="eyebrow">Workspace access</div>
            <p className="mt-3 max-w-lg text-sm leading-7 text-[#3f3f46]">
              Use the same account you already use for your local workspace. This pass changes the shell tone only;
              authentication behavior and routes remain unchanged.
            </p>
          </div>
        </section>

        <section className="flex items-center bg-white px-7 py-8 sm:px-10 sm:py-10">
          <div className="w-full max-w-xl">
            <div className="space-y-2 border-b border-[#e4e4e4] pb-6">
              <div className="eyebrow">{isLogin ? "Return to workspace" : "Create account"}</div>
              <h2 className="text-[2rem] font-semibold leading-[1.04] tracking-[-0.05em] text-[#111111]">
                {isLogin ? "Welcome back" : "Create workspace access"}
              </h2>
              <p className="max-w-md text-sm leading-7 text-[#52525b]">
                {isLogin
                  ? "Sign in to continue working across your notes, sources, and connected context."
                  : "Create an account to open a new workspace session and start collecting material."}
              </p>
            </div>
            <div className="pt-7">
              <form
                className="space-y-5"
                onSubmit={(event) => {
                  event.preventDefault();
                  onSubmit();
                }}
              >
                <div className="space-y-2">
                  <label className="eyebrow">Email</label>
                  <Input value={email} onChange={(event) => onEmailChange(event.target.value)} type="email" required />
                </div>
                <div className="space-y-2">
                  <label className="eyebrow">Password</label>
                  <Input
                    value={password}
                    onChange={(event) => onPasswordChange(event.target.value)}
                    type="password"
                    required
                  />
                </div>
                {errorMessage ? (
                  <div className="rounded-[12px] border border-[#111111] bg-white px-4 py-3 text-sm leading-6 text-[#111111]">
                    {errorMessage}
                  </div>
                ) : null}
                <Button className="w-full" disabled={pending}>
                  {pending ? (
                    <>
                      <span className="mr-2 h-4 w-4 animate-spin rounded-full border-[1.5px] border-current border-r-transparent" />
                      {isLogin ? "Signing in" : "Creating account"}
                    </>
                  ) : isLogin ? (
                    "Sign in"
                  ) : (
                    "Create account"
                  )}
                </Button>
              </form>
              <div className="mt-6 text-sm leading-7 text-[#52525b]">
                {isLogin ? "Need an account?" : "Already have an account?"}{" "}
                <Link
                  className="font-medium text-[#111111] underline underline-offset-4"
                  to={isLogin ? "/register" : "/login"}
                >
                  {isLogin ? "Register" : "Sign in"}
                </Link>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
