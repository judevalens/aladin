import { History } from "lucide-react";
import { useState } from "react";
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
  const { runtime, services } = useAppComposition();
  const authState = useObservableState(services.auth.session.session());
  // Hook before the early return (rules of hooks).
  const [historyOpen, setHistoryOpen] = useState(false);

  const user = authState.status === "data" ? authState.value.user : null;
  if (!user) {
    return (
      <div className="px-7 py-6 text-sm text-gray-700">Loading page…</div>
    );
  }

  const token = runtime.desktopSession.getToken() ?? "";

  return (
    <div className="relative flex h-full min-h-0 flex-col">
      <button
        onClick={() => setHistoryOpen((v) => !v)}
        title="Edit history"
        className="absolute right-3 top-2 z-10 flex items-center gap-1.5 rounded-md border border-[#e7e5e4] bg-white/90 px-2.5 py-1 text-xs font-medium text-[#57534e] shadow-sm hover:bg-[#faf9f8]"
      >
        <History className="h-3.5 w-3.5" />
        History
      </button>
      <div className="min-h-0 flex-1 overflow-hidden">
        <div className="mx-auto flex h-full w-full max-w-workspace-max flex-col">
          <BlockNotePageEditorDriver
            key={pageId}
            pageId={pageId}
            collabWsUrl={runtime.config.collabWsBaseUrl}
            token={token}
            user={{ name: user.email, color: colorForUser(user.id) }}
          />
        </div>
      </div>
      {historyOpen ? (
        <PageHistoryPanel pageId={pageId} onClose={() => setHistoryOpen(false)} />
      ) : null}
    </div>
  );
}
