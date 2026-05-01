import {useEffect} from "react";
import {Bridge} from "./embed";
import {MilkdownEditor} from "./MilkdownEditor";
import "./styles.css";

export type ArtifactSpaProps = {
    widgetId?: string;
    overlayRoot?: HTMLElement | null;
    bridge: Bridge | undefined;
};

function initialMarkdown(widgetId?: string): string {
    if (!widgetId) {
        return "Start writing here.";
    }

    return `# ${widgetId}\n\nStart writing here.`;
}

export default function App({bridge, widgetId}: ArtifactSpaProps) {
    useEffect(() => {
        if (!bridge) return;

        bridge.kotlinEvent = (event) => {
            console.log("Kotlin event received:", event);
        };

        return () => {
            bridge.kotlinEvent = () => {
            };
        };
    }, [bridge]);

    return (
        <div
            className="app-shell"
            style={{
                height: "100%",
                boxSizing: "border-box",
                overflowY: "auto",
            }}
        >
            <MilkdownEditor defaultValue={initialMarkdown(widgetId)} bridge={bridge}/>
        </div>
    );
}
