import {Bridge} from "./embed";
import {MilkdownEditor} from "./MilkdownEditor";
import "./styles.css";

export type ArtifactSpaProps = {
    pageId: string;
    initialMarkdown: string;
    isEditable: boolean;
    overlayRoot?: HTMLElement | null;
    bridge: Bridge | undefined;
};

export default function App({bridge, pageId, initialMarkdown, isEditable}: ArtifactSpaProps) {
    return (
        <div
            className="app-shell"
            style={{
                height: "100%",
                boxSizing: "border-box",
                overflowY: "auto",
            }}
        >
            <MilkdownEditor
                key={pageId}
                pageId={pageId}
                defaultValue={initialMarkdown}
                isEditable={isEditable}
                bridge={bridge}
            />
        </div>
    );
}
