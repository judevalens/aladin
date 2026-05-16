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
import {
  BrowserPaneView,
  PlaceholderDestinationView,
  RenameDialogView,
  VoiceCaptureDialogView,
  WorkPaneView,
  WorkspaceShellView,
} from "@/modules/workspace/ui/workspace-views";
import { PageEditorScreen } from "@/modules/pages/screens";

export function WorkspaceShellLayoutScreen() {
  const shell = useWorkspaceShell();

  return (
    <WorkspaceShellView
      selectedDestination={shell.selectedDestination}
      userEmail={shell.userEmail}
      logoutPending={shell.logoutPending}
      onNavigate={shell.onNavigate}
      onLogout={shell.onLogout}
      onCreateFolder={shell.onCreateFolder}
      onCreateNote={shell.onCreateNote}
      onCreateLink={shell.onCreateLink}
      onCreateVoice={shell.onCreateVoice}
    >
      <Outlet />
    </WorkspaceShellView>
  );
}

export function WorkspaceRouteScreen(_props: { destination: "home" | "folders" }) {
  const browser = useBrowserPane();
  const workPane = useWorkPane();
  const renameDialog = useRenameDialog();
  const voiceDraft = useVoiceDraft();

  return (
    <>
      <BrowserPaneView
        loading={browser.loading}
        errorMessage={browser.errorMessage}
        breadcrumb={browser.breadcrumb}
        rows={browser.rows}
        activeArtifactId={browser.activeArtifactId}
        expandedFolderIds={browser.expandedFolderIds}
        onNavigateToScope={browser.onNavigateToScope}
        onFolderPrimaryAction={browser.onFolderPrimaryAction}
        onSelectFolder={browser.onSelectFolder}
        onOpenArtifact={browser.onOpenArtifact}
        onStartRenameFolder={browser.onStartRenameFolder}
        onStartRenameArtifact={browser.onStartRenameArtifact}
        onCreateFolderHere={browser.onCreateFolderHere}
        onCreateNoteHere={browser.onCreateNoteHere}
      />
      <WorkPaneView
        openArtifacts={workPane.openArtifacts}
        activeArtifact={workPane.activeArtifact}
        statusPath={workPane.statusPath}
        inspectorOpen={workPane.inspectorOpen}
        onActivateArtifact={workPane.onActivateArtifact}
        onCloseArtifact={workPane.onCloseArtifact}
        onToggleInspector={workPane.onToggleInspector}
      >
        {!workPane.activeArtifact ? (
          <PlaceholderPane
            title="Open a page"
            body="Choose a note, link, voice note, or file from the browser to continue."
            className="h-full"
          />
        ) : workPane.activeArtifact.kind === "note" ? (
          <PageEditorScreen pageId={workPane.activeArtifact.id} />
        ) : workPane.activeArtifact.kind === "link" ? (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <LinkArtifactView artifact={workPane.activeArtifact} />
            </div>
          </ScrollArea>
        ) : workPane.activeArtifact.kind === "voice" ? (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <VoiceArtifactView artifact={workPane.activeArtifact} />
            </div>
          </ScrollArea>
        ) : (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <FileArtifactView artifact={workPane.activeArtifact} />
            </div>
          </ScrollArea>
        )}
      </WorkPaneView>
      <RenameDialogView
        rename={renameDialog.rename}
        pending={renameDialog.pending}
        onDraftTitleChange={renameDialog.onDraftTitleChange}
        onCancel={renameDialog.onCancel}
        onSave={renameDialog.onSave}
      />
      <VoiceCaptureDialogView
        draft={voiceDraft.draft}
        permissionError={voiceDraft.permissionError}
        pending={voiceDraft.pending}
        onClose={voiceDraft.onClose}
        onPatchDraft={voiceDraft.onPatchDraft}
        onStartRecording={voiceDraft.onStartRecording}
        onStopRecording={voiceDraft.onStopRecording}
        onSave={voiceDraft.onSave}
      />
    </>
  );
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
