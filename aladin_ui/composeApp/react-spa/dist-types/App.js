import { jsx as _jsx } from "react/jsx-runtime";
import { useEffect } from "react";
import { MilkdownEditor } from "./MilkdownEditor";
import "./styles.css";
function initialMarkdown(widgetId) {
    if (!widgetId) {
        return "Start writing here.";
    }
    return `# ${widgetId}\n\nStart writing here.`;
}
export default function App({ bridge, widgetId }) {
    useEffect(() => {
        if (!bridge)
            return;
        bridge.kotlinEvent = (event) => {
            console.log("Kotlin event received:", event);
        };
        return () => {
            bridge.kotlinEvent = () => {
            };
        };
    }, [bridge]);
    return (_jsx("div", { className: "app-shell", style: {
            height: "100%",
            boxSizing: "border-box",
            overflowY: "auto",
        }, children: _jsx(MilkdownEditor, { defaultValue: initialMarkdown(widgetId), bridge: bridge }) }));
}
