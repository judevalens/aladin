import { jsx as _jsx } from "react/jsx-runtime";
import { useLayoutEffect, useRef } from "react";
import { Crepe } from "@milkdown/crepe";
import "@milkdown/crepe/theme/common/style.css";
import "@milkdown/crepe/theme/frame.css";
function initialMarkdown(title, kind) {
    return `Start writing here.`;
}
export function MilkdownEditor({}) {
    const rootRef = useRef(null);
    useLayoutEffect(() => {
        const root = rootRef.current;
        if (!root)
            return;
        root.innerHTML = "";
        const crepe = new Crepe({
            root,
        });
        void crepe.create();
        return () => {
            void crepe.destroy();
            root.innerHTML = "";
        };
    }, []);
    return _jsx("div", { className: "milkdown-root", ref: rootRef });
}
