import { Outlet } from "react-router-dom";
import { ScrollArea } from "@/components/ui/scroll-area";
import { PlaceholderPane } from "@/components/ui/aladin";
import { FileArtifactView, LinkArtifactView, VoiceArtifactView } from "@/modules/artifacts/ui/artifact-views";
import {
  useBrowserPane,
  useRenameDialog,
  useVoiceDraft,
  useWorkPane,
  useWorkspaceShell,
} from "@/modules/workspace/hooks/use-workspace";
import { BrowserPaneView } from "@/modules/workspace/ui/browser-pane-view";
import { RenameDialogView } from "@/modules/workspace/ui/rename-dialog-view";
import { VoiceCaptureDialogView } from "@/modules/workspace/ui/voice-capture-dialog-view";
import { WorkPaneView } from "@/modules/workspace/ui/work-pane-view";
import {
  PlaceholderDestinationView,
  WorkspaceShellView,
} from "@/modules/workspace/ui/workspace-shell-view";
import { PageEditorScreen } from "@/modules/pages/screens";

export function WorkspaceShellLayoutScreen() {
  const state = useWorkspaceShell();

  return (
    <WorkspaceShellView state={state}>
      <Outlet />
    </WorkspaceShellView>
  );
}

export function WorkspaceRouteScreen(_props: { destination: "home" | "folders" }) {
  return (
    <>
      <BrowserPaneSurface />
      <WorkPaneSurface />
      <RenameDialogSurface />
      <VoiceCaptureDialogSurface />
    </>
  );
}

function BrowserPaneSurface() {
  const state = useBrowserPane();
  return <BrowserPaneView state={state} />;
}

function WorkPaneSurface() {
  const state = useWorkPane();

  return (
    <WorkPaneView state={state}>
      {!state.activeArtifact ? (
        <PlaceholderPane
          title="Open a page"
          body="Choose a note, link, voice note, or file from the browser to continue."
          className="h-full"
        />
      ) : state.activeArtifact.kind === "note" ? (
        <PageEditorScreen pageId={state.activeArtifact.id} />
      ) : state.activeArtifact.kind === "link" ? (
        <ScrollArea className="h-full">
          <div className="mx-auto w-full max-w-workspace-max">
            <LinkArtifactView artifact={state.activeArtifact} />
          </div>
        </ScrollArea>
      ) : state.activeArtifact.kind === "voice" ? (
        <ScrollArea className="h-full">
          <div className="mx-auto w-full max-w-workspace-max">
            <VoiceArtifactView artifact={state.activeArtifact} />
          </div>
        </ScrollArea>
      ) : (
        <ScrollArea className="h-full">
          <div className="mx-auto w-full max-w-workspace-max">
            <FileArtifactView artifact={state.activeArtifact} />
          </div>
        </ScrollArea>
      )}
    </WorkPaneView>
  );
}

function RenameDialogSurface() {
  const state = useRenameDialog();
  return <RenameDialogView state={state} />;
}

function VoiceCaptureDialogSurface() {
  const state = useVoiceDraft();
  return <VoiceCaptureDialogView state={state} />;
}

export function WorkspacePlaceholderScreen({
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
    <PlaceholderDestinationView
      paneTitle={paneTitle}
      paneBody={paneBody}
      workTitle={workTitle}
      workBody={workBody}
    />
  );
}
