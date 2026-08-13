import {
  Code2,
  FileJson,
  FileText,
  FlaskConical,
  History,
  LayoutDashboard,
  Layout,
  Link2,
  Mic,
  Paperclip,
  ScanSearch,
  type LucideIcon,
} from "lucide-react";

import type { ResearchView } from "@/modules/workspace/domain";
import type { ArtifactKind } from "@/shared/api/models";

/**
 * The one place a kind becomes a glyph.
 *
 * These maps used to be private to `browser-pane-ui.tsx`. The tab switcher shows the same
 * kinds in a different surface, and two copies of a kind→glyph map is how a `voice` note
 * ends up with a microphone in the tree and a paperclip in the switcher.
 */
export const ARTIFACT_ICONS: Record<ArtifactKind, LucideIcon> = {
  note: FileText,
  link: Link2,
  voice: Mic,
  file: Paperclip,
  app: Layout,
};

/** A research folder's structural slots (domain.RESEARCH_SLOT_VIEWS) as glyphs. */
export const RESEARCH_SLOT_ICONS: Record<ResearchView, LucideIcon> = {
  overview: LayoutDashboard,
  manifest: FileJson,
  runs: History,
  code: Code2,
  inspect: ScanSearch,
};

/** The research folder itself — the flask, always in amber. */
export const RESEARCH_ICON = FlaskConical;
