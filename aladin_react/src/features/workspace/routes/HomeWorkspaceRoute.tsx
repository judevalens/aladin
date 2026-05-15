import { BrowserPane } from "@/features/workspace/components/BrowserPane";
import { RenameDialog } from "@/features/workspace/components/RenameDialog";
import { VoiceCaptureDialog } from "@/features/workspace/components/VoiceCaptureDialog";
import { WorkPane } from "@/features/workspace/components/WorkPane";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";

interface HomeWorkspaceRouteProps {
  destination: "home" | "folders";
}

export function HomeWorkspaceRoute(_props: HomeWorkspaceRouteProps) {
  const { state } = useWorkspaceShell();

  return (
    <>
      <BrowserPane activeArtifactId={state.activeArtifactId} />
      <div className="w-px bg-aladin-divider" />
      <WorkPane />
      <RenameDialog />
      <VoiceCaptureDialog />
    </>
  );
}
