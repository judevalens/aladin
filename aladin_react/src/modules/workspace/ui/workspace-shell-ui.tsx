import {
  CandlestickChart,
  Check,
  Command,
  Contrast,
  Folder,
  Globe,
  LogOut,
  Network,
  Plus,
  Sparkles,
  SquareTerminal,
} from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect } from "react";
import { Outlet } from "react-router-dom";
import { CommandPalette } from "@/modules/workspace/ui/command-palette";
import { TabSwitcher } from "@/modules/workspace/ui/tab-switcher";
import { canSwitchTabs } from "@/modules/workspace/hooks/use-tab-switcher";
import { TickerModal } from "@/modules/markets/ui/ticker-modal";
import { LinkCaptureDialogUI } from "@/modules/workspace/ui/link-capture-dialog-ui";
import { FileUploadDialogUI } from "@/modules/workspace/ui/file-upload-dialog-ui";
import { TerminalDockUI } from "@/modules/terminal/ui/terminal-dock-ui";
import { CopilotDockUI } from "@/modules/copilot/ui/copilot-dock-ui";
import { NotificationBell, NotificationToast } from "@/modules/notifications/ui/notification-bell-ui";
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
import { THEMES } from "@/app/state/theme-slice";
import { cn } from "@/lib/utils";

const navItems = [
  { key: "markets", label: "Markets", icon: CandlestickChart, path: "/markets" },
  { key: "folders", label: "Research", icon: Folder, path: "/home" },
  { key: "entities", label: "Entities", icon: Globe, path: "/entities" },
  { key: "sources", label: "Sources", icon: Network, path: "/sources" },
] as const;

export function WorkspaceShellUI() {
  const {
    selectedDestination,
    userEmail,
    logoutPending,
    onNavigate,
    onLogout,
    onCreateFolder,
    onCreateResearch,
    onCreateNote,
    onCreateBoard,
    onCreateLink,
    onCreateVoice,
    onCreateFile,
    linkDialogOpen,
    onCloseLinkDialog,
    onSubmitLink,
    fileDialogOpen,
    onCloseFileDialog,
    onSubmitFile,
    createPending,
  } = useWorkspaceShell();
  const theme = useAppStore((state) => state.theme);
  const setTheme = useAppStore((state) => state.setTheme);
  const commandOpen = useAppStore((state) => state.commandPaletteOpen);
  const setCommandOpen = useAppStore((state) => state.setCommandPaletteOpen);
  const terminalOpen = useAppStore((state) => state.terminalOpen);
  const toggleTerminal = useAppStore((state) => state.toggleTerminal);
  const copilotOpen = useAppStore((state) => state.copilotOpen);
  const toggleCopilot = useAppStore((state) => state.toggleCopilot);
  const tabSwitcherOpen = useAppStore((state) => state.tabSwitcherOpen);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        const store = useAppStore.getState();
        store.setCommandPaletteOpen(!store.commandPaletteOpen);
      }
      // Ctrl+` / ⌘` toggles the terminal dock (IDE convention).
      if (event.key === "`" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        useAppStore.getState().toggleTerminal();
      }
      // Ctrl+J / ⌘J toggles the Copilot dock.
      if (event.key.toLowerCase() === "j" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        useAppStore.getState().toggleCopilot();
      }
      // Ctrl+Tab opens the MRU tab switcher. CTRL, not ⌘, on both platforms: ⌘Tab belongs to
      // the OS, and Ctrl+Tab is the IDE convention. preventDefault so focus doesn't walk the
      // DOM. Once open, the overlay owns Tab — this only handles opening it.
      if (event.key === "Tab" && event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        const store = useAppStore.getState();
        if (store.tabSwitcherOpen) return;
        if (!canSwitchTabs(store.workspace.openTabs)) return;
        store.setTabSwitcherOpen(true);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <TooltipProvider delayDuration={200}>
      <CommandPalette
        open={commandOpen}
        onOpenChange={setCommandOpen}
        actions={{ onCreateFolder, onCreateResearch, onCreateNote, onCreateBoard, onCreateLink, onCreateVoice, onCreateFile }}
      />
      {tabSwitcherOpen ? <TabSwitcher /> : null}
      <TickerModal />
      <LinkCaptureDialogUI
        open={linkDialogOpen}
        pending={createPending}
        onOpenChange={(open) => !open && onCloseLinkDialog()}
        onSubmit={onSubmitLink}
      />
      <FileUploadDialogUI
        open={fileDialogOpen}
        pending={createPending}
        onOpenChange={(open) => !open && onCloseFileDialog()}
        onSubmit={onSubmitFile}
      />
      {/* border-t defines the boundary between the OS title bar and the app content, so the
          dark native bar reads as separate window chrome instead of blending into bg. */}
      <div className="flex h-screen overflow-hidden border-t border-line bg-bg font-sans text-ink">
        {/* Activity rail */}
        <nav className="flex w-[52px] shrink-0 flex-col items-center gap-1 border-r border-line bg-rail py-3">
            <button
              type="button"
              onClick={() => setCommandOpen(true)}
              className="grid size-8 place-items-center rounded-control bg-amber font-display text-body font-bold text-primary-foreground"
              aria-label="Open command palette"
            >
              A
            </button>

            <DropdownMenu>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="mt-1 grid size-[38px] place-items-center rounded-control text-ink-3 transition-colors hover:bg-hover hover:text-ink"
                      aria-label="Capture"
                    >
                      <Icon as={Plus} size="rail" className="text-amber" />
                    </button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent side="right">Capture</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="start" side="right" className="w-52">
                <DropdownMenuLabel>Create</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={onCreateFolder}>New folder</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateResearch}>New research</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateNote}>New note</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateBoard}>New board</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateLink}>New link</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateVoice}>New voice note</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateFile}>Upload file</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <div className="mt-1 flex flex-col items-center gap-1">
              {navItems.map((item) => {
                const Glyph = item.icon;
                const active = item.key === selectedDestination;
                return (
                  <Tooltip key={item.key}>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        onClick={() => onNavigate(item.path)}
                        className={cn(
                          "relative grid size-[38px] place-items-center rounded-control transition-colors",
                          active
                            ? "bg-sel text-ink"
                            : "text-ink-3 hover:bg-hover hover:text-ink",
                        )}
                        aria-label={item.label}
                        aria-current={active ? "page" : undefined}
                      >
                        <Icon as={Glyph} size="rail" />
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
                    onClick={toggleTerminal}
                    className={cn(
                      "grid size-[38px] place-items-center rounded-control transition-colors",
                      terminalOpen
                        ? "bg-sel text-ink"
                        : "text-ink-3 hover:bg-hover hover:text-ink",
                    )}
                    aria-label="Toggle terminal"
                    aria-pressed={terminalOpen}
                  >
                    <Icon as={SquareTerminal} size="rail" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right">Terminal · ⌘`</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    onClick={toggleCopilot}
                    className={cn(
                      "grid size-[38px] place-items-center rounded-control transition-colors",
                      copilotOpen
                        ? "bg-sel text-ink"
                        : "text-ink-3 hover:bg-hover hover:text-ink",
                    )}
                    aria-label="Toggle copilot"
                    aria-pressed={copilotOpen}
                  >
                    <Icon as={Sparkles} size="rail" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right">Copilot · ⌘J</TooltipContent>
              </Tooltip>

              <NotificationBell />

              <DropdownMenu>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <DropdownMenuTrigger asChild>
                      <button
                        type="button"
                        className="grid size-[38px] place-items-center rounded-control text-ink-3 transition-colors hover:bg-hover hover:text-ink aria-expanded:bg-sel aria-expanded:text-ink"
                        aria-label="Theme"
                      >
                        <Icon as={Contrast} />
                      </button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent side="right">Theme</TooltipContent>
                </Tooltip>
                <DropdownMenuContent side="right" align="end" className="w-52">
                  <DropdownMenuLabel>Theme</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {THEMES.map((t) => (
                    <DropdownMenuItem
                      key={t.name}
                      onClick={() => setTheme(t.name)}
                      className="flex items-start gap-2"
                    >
                      <Icon as={Check} size="inline" mark className={cn("mt-0.5 shrink-0", theme === t.name ? "text-amber" : "text-transparent", )} />
                      <span className="flex min-w-0 flex-col">
                        <span className="text-body text-ink">{t.label}</span>
                        <span className="font-mono text-meta text-ink-4">{t.hint}</span>
                      </span>
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>

              <DropdownMenu>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <DropdownMenuTrigger asChild>
                      <button
                        type="button"
                        className="grid size-8 place-items-center rounded-full border border-line bg-field text-meta font-semibold uppercase text-ink-2 transition-colors hover:text-ink"
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
                    <Icon as={LogOut} className="mr-2]" />
                    Sign out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>

              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    onClick={() => setCommandOpen(true)}
                    className="grid size-[38px] place-items-center rounded-control text-ink-3 transition-colors hover:bg-hover hover:text-ink"
                    aria-label="Command palette"
                  >
                    <Icon as={Command} />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right">Command · ⌘K</TooltipContent>
              </Tooltip>
            </div>
          </nav>

        <main className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
          <div className="flex min-h-0 flex-1 overflow-hidden">
            <Outlet />
          </div>
          <TerminalDockUI />
        </main>
        <CopilotDockUI />
        <NotificationToast />
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
