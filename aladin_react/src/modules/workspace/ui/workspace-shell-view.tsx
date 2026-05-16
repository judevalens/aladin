import { Folder, GitGraph, Home, LogOut, Network, Plus, Signal } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AladinToolbarField, PlaceholderPane } from "@/components/ui/aladin";
import { cn } from "@/shared/lib/utils";

const navItems = [
  { key: "home", label: "Home", icon: Home, path: "/home" },
  { key: "folders", label: "Folders", icon: Folder, path: "/folders" },
  { key: "signals", label: "Signals", icon: Signal, path: "/signals" },
  { key: "sources", label: "Sources", icon: Network, path: "/sources" },
  { key: "graph", label: "Graph", icon: GitGraph, path: "/graph" },
] as const;

export function WorkspaceShellView({
  selectedDestination,
  userEmail,
  logoutPending,
  onNavigate,
  onLogout,
  onCreateFolder,
  onCreateNote,
  onCreateLink,
  onCreateVoice,
  children,
}: {
  selectedDestination: string;
  userEmail: string;
  logoutPending: boolean;
  onNavigate: (path: string) => void;
  onLogout: () => void;
  onCreateFolder: () => void;
  onCreateNote: () => void;
  onCreateLink: () => void;
  onCreateVoice: () => void;
  children: ReactNode;
}) {
  return (
    <div className="app-canvas flex h-screen text-[#0a0a0a]">
      <aside className="flex h-full w-[252px] shrink-0 border-r border-[#e7e5e4] bg-white sm:w-[272px] lg:w-sidebar">
        <div className="flex min-h-0 w-full flex-col">
          <div className="px-4 pt-5 pb-4">
            <div className="flex items-center gap-2.5">
              <div className="flex h-7 w-7 items-center justify-center rounded-md bg-[#18181b] text-[12px] font-semibold text-[#fafaf9]">
                A
              </div>
              <div className="min-w-0">
                <div className="text-[13px] font-semibold tracking-[-0.01em] text-[#0a0a0a]">Aladin</div>
                <div className="text-[11px] leading-4 text-[#78716c]">Workspace memory</div>
              </div>
            </div>
          </div>

          <div className="space-y-2 px-3">
            <AladinToolbarField text="Search…" />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="default" size="sm" className="w-full justify-between">
                  <span>New item</span>
                  <Plus className="h-3.5 w-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-52">
                <DropdownMenuLabel>Create</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={onCreateFolder}>New folder</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateNote}>New note</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateLink}>New link</DropdownMenuItem>
                <DropdownMenuItem onClick={onCreateVoice}>New voice note</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <div className="mt-5 px-3">
            <div className="eyebrow px-2 pb-2">Browse</div>
            <nav className="space-y-0.5">
              {navItems.map((item) => {
                const Icon = item.icon;
                const active = item.key === selectedDestination;
                return (
                  <button
                    key={item.key}
                    className={cn(
                      "flex h-8 w-full items-center gap-2.5 rounded-md px-2 text-[13px] transition-colors",
                      active
                        ? "bg-[#ebebe8] font-medium text-[#0a0a0a]"
                        : "text-[#57534e] hover:bg-[#f2f0ee] hover:text-[#0a0a0a]",
                    )}
                    onClick={() => onNavigate(item.path)}
                    type="button"
                  >
                    <Icon className="h-[15px] w-[15px]" strokeWidth={1.75} />
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          <div className="mt-auto border-t border-[#e7e5e4] px-3 py-3">
            <div className="truncate px-2 text-[12px] text-[#78716c]">{userEmail}</div>
            <button
              className="mt-1 flex h-8 w-full items-center gap-2.5 rounded-md px-2 text-[13px] text-[#57534e] transition-colors hover:bg-[#f2f0ee] hover:text-[#0a0a0a] disabled:opacity-50"
              onClick={onLogout}
              type="button"
              disabled={logoutPending}
            >
              <LogOut className="h-[15px] w-[15px]" strokeWidth={1.75} />
              <span>Sign out</span>
            </button>
          </div>
        </div>
      </aside>
      <main className="flex min-w-0 flex-1 overflow-hidden">{children}</main>
    </div>
  );
}

export function PlaceholderDestinationView({
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
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-[#e7e5e4] bg-white sm:w-[368px]">
        <PlaceholderPane title={paneTitle} body={paneBody} className="h-full" />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-white">
        <PlaceholderPane title={workTitle} body={workBody} className="h-full" />
      </section>
    </>
  );
}
