import { History, Sparkles } from "lucide-react";
import { useRef, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import { PageHistoryPanel } from "@/modules/pages/ui/page-history-panel";
import { useObservableState } from "@/shared/flow/use-observable-state";

// Stable, readable cursor color per user id (awareness).
function colorForUser(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i += 1) {
    hash = (hash * 31 + id.charCodeAt(i)) | 0;
  }
  const hue = Math.abs(hash) % 360;
  return `hsl(${hue} 70% 45%)`;
}

export function PageEditorUI({ pageId }: { pageId: string }) {
  const { runtime, services, repos } = useAppComposition();
  const authState = useObservableState(services.auth.session.session());
  // Hooks before the early return (rules of hooks).
  const [historyOpen, setHistoryOpen] = useState(false);
  const getPlainText = useRef<(() => string) | null>(null);
  const [analyzing, setAnalyzing] = useState(false);
  const [analyzeMsg, setAnalyzeMsg] = useState<string | null>(null);

  async function analyze() {
    const getText = getPlainText.current;
    if (!getText || analyzing) return;
    setAnalyzing(true);
    setAnalyzeMsg(null);
    try {
      const { claimsStored } = await repos.graphPane.extractClaims(pageId, getText());
      setAnalyzeMsg(
        claimsStored > 0
          ? `${claimsStored} claim${claimsStored === 1 ? "" : "s"} extracted`
          : "No new claims",
      );
    } catch {
      setAnalyzeMsg("Analyze failed");
    } finally {
      setAnalyzing(false);
    }
  }

  const user = authState.status === "data" ? authState.value.user : null;
  if (!user) {
    return (
      <div className="px-7 py-6 text-sm text-ink-2">Loading page…</div>
    );
  }

  const token = runtime.desktopSession.getToken() ?? "";

  return (
    <div className="relative flex h-full min-h-0 flex-col">
      <div className="absolute right-3 top-2 z-10 flex items-center gap-2">
        {analyzeMsg ? <span className="text-xs text-ink-4">{analyzeMsg}</span> : null}
        <button
          onClick={analyze}
          disabled={analyzing}
          title="Extract claims from this page (grounded in its entities)"
          className="flex items-center gap-1.5 rounded-md border border-line bg-field/90 px-2.5 py-1 text-xs font-medium text-ink-2 backdrop-blur-sm hover:bg-raise hover:text-ink disabled:opacity-50"
        >
          <Sparkles className="h-3.5 w-3.5" />
          {analyzing ? "Analyzing…" : "Analyze"}
        </button>
        <button
          onClick={() => setHistoryOpen((v) => !v)}
          title="Edit history"
          className="flex items-center gap-1.5 rounded-md border border-line bg-field/90 px-2.5 py-1 text-xs font-medium text-ink-2 backdrop-blur-sm hover:bg-raise hover:text-ink"
        >
          <History className="h-3.5 w-3.5" />
          History
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden">
        <div className="mx-auto flex h-full w-full max-w-workspace-max flex-col">
          <BlockNotePageEditorDriver
            key={pageId}
            pageId={pageId}
            collabWsUrl={runtime.config.collabWsBaseUrl}
            token={token}
            user={{ name: user.email, color: colorForUser(user.id) }}
            searchEntities={(q) => repos.graphPane.searchEntities(q)}
            createEntity={(name) => repos.graphPane.createEntity(name, "unknown")}
            onMentionsChange={(mentions) =>
              void repos.graphPane.syncMentions(pageId, mentions).catch(() => undefined)
            }
            onReady={(api) => {
              getPlainText.current = api.getPlainText;
            }}
          />
        </div>
      </div>
      {historyOpen ? (
        <PageHistoryPanel pageId={pageId} onClose={() => setHistoryOpen(false)} />
      ) : null}
    </div>
  );
}
