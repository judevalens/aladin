import { jsx as _jsx } from "react/jsx-runtime";
import { MilkdownEditor } from "./MilkdownEditor";
import "./styles.css";
export default function App({ bridge, pageId, initialMarkdown, isEditable }) {
    return (_jsx("div", { className: "app-shell", style: {
            height: "100%",
            boxSizing: "border-box",
            overflowY: "auto",
        }, children: _jsx(MilkdownEditor, { pageId: pageId, defaultValue: initialMarkdown, isEditable: isEditable, bridge: bridge }, pageId) }));
}
