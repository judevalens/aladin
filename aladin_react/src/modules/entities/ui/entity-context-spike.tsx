import { useState } from "react";

import { useAppStore } from "@/app/state/store";
import type { Edge } from "../entity-context-types";
import { CONTEXT, EDGES, ENTITY, SUGGESTED_EDGE } from "../entity-context-mock";
import { EntityContextView } from "./entity-context-ui";

// Dev-only Entity Context spike (route: /spike/entity-context). Renders the
// Phase A surface on mock data, standalone and outside the login wall, so the
// pixel-faithful recreation of design/ENTITY_CONTEXT_PRD.md can be iterated on
// and checked in both themes. Not part of the shipped app.
export function EntityContextSpike() {
  const theme = useAppStore((s) => s.theme);
  // Session-only, per the handoff: Keep commits the suggested edge as FOUND,
  // Dismiss just drops it. The real surface persists via the write path.
  const [edges, setEdges] = useState<Edge[]>(EDGES);
  const [suggestion, setSuggestion] = useState<Edge | null>(SUGGESTED_EDGE);

  return (
    <div className="flex h-screen flex-col bg-bg text-ink">
      <div className="flex items-center gap-3 border-b border-line bg-chrome px-4 py-2 text-small text-ink-3">
        <span className="font-mono">/spike/entity-context</span>
        <span className="text-ink-4">·</span>
        <span>theme: {theme}</span>
        <button
          className="rounded-chip border border-line px-2 py-0.5 hover:bg-raise hover:text-ink"
          onClick={() => useAppStore.getState().setTheme(theme === "dark" ? "soft" : "dark")}
        >
          toggle theme
        </button>
      </div>
      <div className="min-h-0 flex-1">
        <EntityContextView
          entity={ENTITY}
          edges={edges}
          context={CONTEXT}
          suggestion={suggestion}
          onKeepEdge={() => {
            if (!suggestion) return;
            setEdges((prev) => [...prev, suggestion]);
            setSuggestion(null);
          }}
          onDismissSuggestion={() => setSuggestion(null)}
        />
      </div>
    </div>
  );
}
