import { jsx as _jsx } from "react/jsx-runtime";
import { Crepe } from "@milkdown/crepe";
import { Milkdown, MilkdownProvider, useEditor } from "@milkdown/react";
import { useEffect, useRef } from "react";
function EditorSurface({ defaultValue = "Start writing here.", bridge }) {
    const latestMarkdownRef = useRef(defaultValue);
    const lastEmittedMarkdownRef = useRef(defaultValue);
    const flushTimerRef = useRef(null);
    const clearFlushTimer = () => {
        if (flushTimerRef.current !== null) {
            window.clearTimeout(flushTimerRef.current);
            flushTimerRef.current = null;
        }
    };
    const flushDocumentUpdate = () => {
        const markdown = latestMarkdownRef.current;
        clearFlushTimer();
        if (!bridge || markdown === lastEmittedMarkdownRef.current) {
            return;
        }
        bridge.jsEvent({
            type: "documentUpdated",
            payload: {
                markdown,
            },
        });
        lastEmittedMarkdownRef.current = markdown;
    };
    const scheduleFlush = () => {
        clearFlushTimer();
        flushTimerRef.current = window.setTimeout(() => {
            flushDocumentUpdate();
        }, 250);
    };
    useEditor((root) => {
        const crepe = new Crepe({
            root,
            defaultValue,
        });
        crepe.on((listener) => {
            listener.markdownUpdated((_ctx, markdown) => {
                latestMarkdownRef.current = markdown;
                scheduleFlush();
            });
            listener.blur(() => {
                flushDocumentUpdate();
            });
        });
        return crepe;
    }, [defaultValue, bridge]);
    useEffect(() => {
        latestMarkdownRef.current = defaultValue;
        lastEmittedMarkdownRef.current = defaultValue;
        clearFlushTimer();
    }, [defaultValue]);
    useEffect(() => {
        return () => {
            flushDocumentUpdate();
            clearFlushTimer();
        };
    }, []);
    return (_jsx("div", { className: "milkdown-root", style: { height: "100%", overflow: "visible" }, children: _jsx(Milkdown, {}) }));
}
export function MilkdownEditor({ defaultValue, bridge }) {
    return (_jsx(MilkdownProvider, { children: _jsx(EditorSurface, { defaultValue: defaultValue, bridge: bridge }) }));
}
