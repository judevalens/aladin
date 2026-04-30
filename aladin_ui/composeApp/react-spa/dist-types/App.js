import { jsx as _jsx, Fragment as _Fragment } from "react/jsx-runtime";
import { MilkdownEditor } from "./MilkdownEditor";
import { useEffect } from "react";
export default function App({ bridge, overlayRoot = null, }) {
    useEffect(() => {
        if (!bridge)
            return;
        bridge.kotlinEvent = (event) => {
            console.log("JS event received: ", event);
        };
        return () => {
            bridge.kotlinEvent = (event) => {
                console.log(`JS event received: ${event} | NO_OP event handler!`);
            };
        };
    }, [bridge]);
    return (_jsx(_Fragment, { children: _jsx("div", { className: "app-shell", style: {
                height: "100%",
                boxSizing: "border-box",
            }, children: _jsx("div", { style: {
                    height: "100%",
                    paddingLeft: "15px",
                    paddingRight: "15px",
                    boxSizing: "border-box",
                    overflowY: "auto",
                }, children: _jsx(MilkdownEditor, {}) }) }) }));
}
