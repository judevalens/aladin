import type { StateCreator } from "zustand";

// Embedded terminal dock — an IDE-style bottom panel that persists across routes.
// The slice tracks only session METADATA + which tab is active + open/closed. The
// actual xterm ↔ pty binding lives in each mounted TerminalInstance (see
// use-terminal-session). Sessions are ephemeral: they die with the app.
export interface TerminalSessionMeta {
  id: string;
  title: string;
}

export interface TerminalSlice {
  terminalOpen: boolean;
  terminalSessions: TerminalSessionMeta[];
  activeTerminalId: string | null;
  setTerminalOpen: (open: boolean) => void;
  toggleTerminal: () => void;
  addTerminalSession: (session: TerminalSessionMeta) => void;
  removeTerminalSession: (id: string) => void;
  setActiveTerminal: (id: string) => void;
}

export const createTerminalSlice: StateCreator<TerminalSlice, [], [], TerminalSlice> = (set) => ({
  terminalOpen: false,
  terminalSessions: [],
  activeTerminalId: null,
  setTerminalOpen: (open) => set({ terminalOpen: open }),
  toggleTerminal: () => set((state) => ({ terminalOpen: !state.terminalOpen })),
  addTerminalSession: (session) =>
    set((state) =>
      state.terminalSessions.some((s) => s.id === session.id)
        ? { activeTerminalId: session.id }
        : {
            terminalSessions: [...state.terminalSessions, session],
            activeTerminalId: session.id,
          },
    ),
  removeTerminalSession: (id) =>
    set((state) => {
      const index = state.terminalSessions.findIndex((s) => s.id === id);
      if (index === -1) return {};
      const terminalSessions = state.terminalSessions.filter((s) => s.id !== id);
      let activeTerminalId = state.activeTerminalId;
      if (activeTerminalId === id) {
        const neighbor = terminalSessions[index] ?? terminalSessions[index - 1] ?? null;
        activeTerminalId = neighbor?.id ?? null;
      }
      // Closing the last tab collapses the dock.
      return { terminalSessions, activeTerminalId, terminalOpen: terminalSessions.length > 0 && state.terminalOpen };
    }),
  setActiveTerminal: (id) => set({ activeTerminalId: id }),
});
