import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useAppStore } from "@/app/state/store";
import type { CopilotCitation } from "@/app/state/copilot-slice";

/**
 * Routes a copilot citation chip the same way the command palette routes a search hit:
 * ticker → the global ticker modal, page/shard → open the artifact, everything else → the
 * entity context surface.
 */
export function useCitationNav() {
  const navigate = useNavigate();
  const openArtifact = useAppStore((s) => s.openArtifact);
  const openTicker = useAppStore((s) => s.openTicker);

  return useCallback(
    (citation: CopilotCitation) => {
      if (citation.kind === "ticker") {
        openTicker(citation.title || citation.id);
      } else if (citation.kind === "page" || citation.kind === "shard") {
        openArtifact(citation.id);
        navigate("/folders");
      } else {
        navigate(`/entity/${citation.id}`);
      }
    },
    [navigate, openArtifact, openTicker],
  );
}
