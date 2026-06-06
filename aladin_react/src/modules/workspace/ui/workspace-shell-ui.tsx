import { Command, Contrast, Folder, GitGraph, Home, LogOut, Network, Plus, Signal } from "lucide-react";
import { Outlet } from "react-router-dom";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { PlaceholderPane } from "@/components/ui/aladin";
import { useWorkspaceShell } from "@/modules/workspace/hooks/use-workspace-state";
import { useAppStore } from "@/app/state/store";
import { cn } from "@/lib/utils";

const navItems = [
  { key: "home", label: "Home", icon: Home, path: "/home" },
  { key: "folders", label: "Folders", icon: Folder, path: "/folders" },
  { key: "signals", label: "Signals", icon: Signal, path: "/signals" },
  { key: "sources", label: "Sources", icon: Network, path: "/sources" },
  { key: "graph", label: "Graph", icon: GitGraph, path: "/graph" },
] as const;

export function WorkspaceShellUI() {
  const {
    selectedDestination,
    userEmail,
    logoutPending,
    onNavigate,
    onLogout,
    onCreateFolder,
    onCreateNote,
    onCreateLink,
    onCreateVoice,
  } = useWorkspaceShell();
  const theme = useAppStore((state) => state.theme);
  const toggleTheme = useAppStore((state) => state.toggleTheme);

  const activeLabel = navItems.find((item) => item.key === selectedDestination)?.label ?? "Aladin";

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex h-screen flex-col overflow-hidden bg-bg font-sans text-ink">
        {/* Title bar */}
        <header className="relative flex h-10 shrink-0 items-center gap-2 border-b border-line bg-chrome px-3.5">
          <div className="flex items-center gap-2">
            <span className="size-3 rounded-full bg-[#ff5f57]" />
            <span className="size-3 rounded-full bg-[#febc2e]" />
            <span className="size-3 rounded-full bg-[#28c840]" />
          </div>
          <div className="pointer-events-none absolute left-1/2 -translate-x-1/2">
            <div className="flex items-center gap-2 rounded-chip border border-line bg-field px-3 py-1 font-mono text-[11px] text-ink-2">
              <span className="text-ink-3">ʀ</span>
              <span className="text-ink">{activeLabel}</span>
              <kbd className="rounded border border-line px-1 text-[9.5px] text-ink-3">⌘K</kbd>
            </div>
          </div>
        </header>

        <div className="flex min-h-0 flex-1">
          {/* Activity rail */}
          <nav className="flex w-[52px] shrink-0 flex-col items-center gap-1 border-r border-line bg-rail py-3">
            <button
              type="button"
              onClick={() => onNavigate("/home")}
              className="grid size-8 place-items-center rounded-[9px] bg-amber font-display text-sm font-bold text-[#0f0f12]"
              aria-label="Aladin"
            >
              A
            </button>

            <DropdownMenu>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="mt-1 grid size-[38px] place-items-center rounded-[9px] text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                      aria-label="Capture"
                    >
                      <Plus className="size-[18px] text-amber" strokeWidth={1.7} />
                    </button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent side="right">Capture</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="start" side="right" className="w-52">
                <DropdownMenuLabel>Create</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={onCreateFolder}>New folder</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateNote}>New note</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateLink}>New link</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateVoice}>New voice note</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <div className="mt-1 flex flex-col items-center gap-1">
              {navItems.map((item) => {
                const Icon = item.icon;
                const active = item.key === selectedDestination;
                return (
                  <Tooltip key={item.key}>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        onClick={() => onNavigate(item.path)}
                        className={cn(
                          "relative grid size-[38px] place-items-center rounded-[9px] transition-colors",
                          active
                            ? "bg-[rgb(var(--sel))] text-ink"
                            : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
                        )}
                        aria-label={item.label}
                        aria-current={active ? "page" : undefined}
                      >
                        <Icon className="size-[18px]" strokeWidth={1.7} />
                        {item.key === "signals" ? (
                          <span className="absolute right-1.5 top-1.5 size-1.5 rounded-full bg-amber" />
                        ) : null}
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="right">{item.label}</TooltipContent>
                  </Tooltip>
                );
              })}
            </div>

            <div className="mt-auto flex flex-col items-center gap-1">
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    onClick={toggleTheme}
                    className="grid size-[38px] place-items-center rounded-[9px] text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                    aria-label="Toggle theme"
                  >
                    <Contrast className="size-[17px]" strokeWidth={1.7} />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right">{theme === "dark" ? "Soft theme" : "Dark theme"}</TooltipContent>
              </Tooltip>

              <DropdownMenu>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <DropdownMenuTrigger asChild>
                      <button
                        type="button"
                        className="grid size-8 place-items-center rounded-full border border-line bg-field text-[11px] font-semibold uppercase text-ink-2 transition-colors hover:text-ink"
                        aria-label="Account"
                      >
                        {(userEmail ?? "?").slice(0, 1)}
                      </button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent side="right">Account</TooltipContent>
                </Tooltip>
                <DropdownMenuContent align="end" side="right" className="w-56">
                  <DropdownMenuLabel className="truncate font-normal text-ink-2">{userEmail}</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={onLogout} disabled={logoutPending}>
                    <LogOut className="mr-2 size-[15px]" strokeWidth={1.75} />
                    Sign out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>

              <div
                className="grid size-[38px] place-items-center rounded-[9px] text-ink-4"
                aria-hidden="true"
              >
                <Command className="size-[17px]" strokeWidth={1.7} />
              </div>
            </div>
          </nav>

          <main className="flex min-w-0 flex-1 overflow-hidden bg-bg">
            <Outlet />
          </main>
        </div>
      </div>
    </TooltipProvider>
  );
}

export function PlaceholderDestinationUI({
  paneTitle,
  paneBody,
  workTitle,
  workBody,
}: {
  paneTitle: string;
  paneBody: string;
  workTitle: string;
  workBody: string;
}) {
  return (
    <>
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-line bg-explorer sm:w-[368px]">
        <PlaceholderPane title={paneTitle} body={paneBody} className="h-full" />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-bg">
        <PlaceholderPane title={workTitle} body={workBody} className="h-full" />
      </section>
    </>
  );
}
