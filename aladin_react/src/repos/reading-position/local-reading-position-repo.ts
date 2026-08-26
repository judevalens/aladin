import { invoke } from "@tauri-apps/api/core";

import type { LocalReadingPosition } from "@/repos/reading-position/local-reading-position-types";

// Reading position read from the LOCAL `reading_positions` replica (fed by sync
// frames, entity kind "reading_position"). Desktop (Tauri) only: the web host
// falls back to REST behind the same service. Live changes ride the shared
// data-events channel — the service pushes them into its keyed stream.

export interface LocalReadingPositionRepo {
  get(artifactId: string): Promise<LocalReadingPosition | null>;
}

export function createLocalReadingPositionRepo(): LocalReadingPositionRepo {
  return {
    get(artifactId) {
      return invoke<LocalReadingPosition | null>("db_get_reading_position", { artifactId });
    },
  };
}
