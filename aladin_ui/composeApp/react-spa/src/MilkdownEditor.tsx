import {useLayoutEffect, useMemo, useRef} from "react";
import {Crepe} from "@milkdown/crepe";
import "@milkdown/crepe/theme/common/style.css";
import "@milkdown/crepe/theme/frame.css";

type MilkdownEditorProps = {};

function initialMarkdown(title: string, kind: string): string {
    return `Start writing here.`;
}

export function MilkdownEditor({}: MilkdownEditorProps) {
    const rootRef = useRef<HTMLDivElement | null>(null);

    useLayoutEffect(() => {
        const root = rootRef.current;
        if (!root) return;

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

    return <div className="milkdown-root" ref={rootRef}/>;
}
