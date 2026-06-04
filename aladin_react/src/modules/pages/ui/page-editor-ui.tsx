import { useAppComposition } from "@/app/composition/app-composition";
import { BlockNotePageEditorDriver } from "@/modules/pages/editor/page-editor-driver";
import { usePageAttribution } from "@/modules/pages/hooks/use-page-attribution";
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
  // Hook must run before the early return (rules of hooks); pageId is stable.
  const { attribution, refetch } = usePageAttribution(pageId);

  const user = authState.status === "data" ? authState.value.user : null;
  if (!user) {
    return (
      <div className="px-7 py-6 text-sm text-gray-700">Loading page…</div>
    );
  }

  const token = runtime.desktopSession.getToken() ?? "";

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="min-h-0 flex-1 overflow-hidden">
        <div className="mx-auto flex h-full w-full max-w-workspace-max flex-col">
          <BlockNotePageEditorDriver
            key={pageId}
            pageId={pageId}
            collabWsUrl={runtime.config.collabWsBaseUrl}
            token={token}
            user={{ name: user.email, color: colorForUser(user.id) }}
            attribution={attribution}
            onContentChange={refetch}
          />
        </div>
      </div>
    </div>
  );
}
