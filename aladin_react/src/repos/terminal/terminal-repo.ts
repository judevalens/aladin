import { Channel, invoke } from "@tauri-apps/api/core";

// Embedded terminal — the IPC boundary to the Rust PTY layer. Output streams down a
// RAW-BYTES channel (Tauri delivers it as an ArrayBuffer over the binary IPC path —
// no base64/JSON on the hot path); a separate low-frequency control channel carries
// the exit event as JSON. The frontend mints the session id. See
// src-tauri/src/terminal/mod.rs.

export type TerminalControlEvent = { type: "exit"; payload: { code: number | null } };

export interface TerminalOpenOptions {
  cols: number;
  rows: number;
  cwd?: string;
  shell?: string;
}

export interface TerminalRepo {
  open(
    id: string,
    options: TerminalOpenOptions,
    onData: (bytes: Uint8Array) => void,
    onExit: (code: number | null) => void,
  ): Promise<void>;
  write(id: string, data: string): Promise<void>;
  resize(id: string, cols: number, rows: number): Promise<void>;
  close(id: string): Promise<void>;
}

export function createTerminalRepo(): TerminalRepo {
  return {
    open(id, options, onData, onExit) {
      const output = new Channel<ArrayBuffer>((buffer) => onData(new Uint8Array(buffer)));
      const control = new Channel<TerminalControlEvent>((event) => {
        if (event.type === "exit") onExit(event.payload.code);
      });
      return invoke("terminal_open", { id, output, control, input: options });
    },
    write(id, data) {
      return invoke("terminal_write", { id, data });
    },
    resize(id, cols, rows) {
      return invoke("terminal_resize", { id, cols, rows });
    },
    close(id) {
      return invoke("terminal_close", { id });
    },
  };
}

// Stateless invoke-wrapper — a module singleton is safe and avoids threading it
// through the composition root for a surface with no cross-repo coupling.
export const terminalRepo = createTerminalRepo();
