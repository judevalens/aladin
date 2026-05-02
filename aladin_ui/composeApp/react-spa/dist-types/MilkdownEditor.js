import { jsx as _jsx } from "react/jsx-runtime";
import { Crepe } from "@milkdown/crepe";
import { Milkdown, MilkdownProvider, useEditor } from "@milkdown/react";
import { useEffect, useRef, } from "react";
async function fileToBase64(file) {
    const bytes = new Uint8Array(await file.arrayBuffer());
    let binary = "";
    const chunkSize = 0x8000;
    for (let index = 0; index < bytes.length; index += chunkSize) {
        const chunk = bytes.subarray(index, index + chunkSize);
        binary += String.fromCharCode(...chunk);
    }
    return window.btoa(binary);
}
function createRequestId() {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
        return crypto.randomUUID();
    }
    return `upload-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
function EditorSurface({ pageId, defaultValue = "Start writing here.", isEditable, bridge }) {
    const latestMarkdownRef = useRef(defaultValue);
    const lastEmittedMarkdownRef = useRef(defaultValue);
    const flushTimerRef = useRef(null);
    const pendingUploadsRef = useRef(new Map());
    const crepeRef = useRef(null);
    const isEditableRef = useRef(isEditable);
    isEditableRef.current = isEditable;
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
                pageId,
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
    const preventReadonlyInput = (event) => {
        if (!isEditableRef.current) {
            event.preventDefault();
        }
    };
    const preventReadonlyKey = (event) => {
        if (isEditableRef.current) {
            return;
        }
        const isTextInput = event.key.length === 1 && !event.metaKey && !event.ctrlKey && !event.altKey;
        const isEditingKey = ["Backspace", "Delete", "Enter", "Tab"].includes(event.key);
        const isFormattingShortcut = (event.metaKey || event.ctrlKey) && ["b", "i", "u", "k", "z", "y"].includes(event.key.toLowerCase());
        if (isTextInput || isEditingKey || isFormattingShortcut) {
            event.preventDefault();
        }
    };
    useEffect(() => {
        if (!bridge)
            return;
        const handleBridgeEvent = (event) => {
            if (event.type === "fileUploadCompleted") {
                const { requestId, url } = event.payload;
                const pendingUpload = pendingUploadsRef.current.get(requestId);
                if (!pendingUpload)
                    return;
                pendingUploadsRef.current.delete(requestId);
                pendingUpload.resolve(url);
                return;
            }
            if (event.type === "fileUploadFailed") {
                const { requestId, message } = event.payload;
                const pendingUpload = pendingUploadsRef.current.get(requestId);
                if (!pendingUpload)
                    return;
                pendingUploadsRef.current.delete(requestId);
                pendingUpload.reject(new Error(message));
            }
        };
        bridge.kotlinEvent = handleBridgeEvent;
        return () => {
            bridge.kotlinEvent = () => {
            };
        };
    }, [bridge]);
    useEditor((root) => {
        const crepe = new Crepe({
            root,
            defaultValue,
            featureConfigs: {
                [Crepe.Feature.ImageBlock]: {
                    onUpload: async (file) => {
                        if (!bridge) {
                            throw new Error("Upload bridge unavailable.");
                        }
                        const requestId = createRequestId();
                        const base64Data = await fileToBase64(file);
                        const uploadPromise = new Promise((resolve, reject) => {
                            pendingUploadsRef.current.set(requestId, { resolve, reject });
                        });
                        bridge.jsEvent({
                            type: "fileUploadRequested",
                            payload: {
                                pageId,
                                requestId,
                                filename: file.name,
                                contentType: file.type || undefined,
                                base64Data,
                            },
                        });
                        return uploadPromise;
                    },
                },
            },
        });
        crepe.setReadonly(!isEditableRef.current);
        crepeRef.current = crepe;
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
    }, [bridge, pageId]);
    useEffect(() => {
        if (!isEditable) {
            flushDocumentUpdate();
        }
        crepeRef.current?.setReadonly(!isEditable);
    }, [isEditable]);
    useEffect(() => {
        return () => {
            flushDocumentUpdate();
            clearFlushTimer();
            pendingUploadsRef.current.forEach(({ reject }) => {
                reject(new Error("Editor unmounted before upload finished."));
            });
            pendingUploadsRef.current.clear();
            crepeRef.current = null;
        };
    }, []);
    return (_jsx("div", { className: `milkdown-root${isEditable ? "" : " milkdown-root--readonly"}`, style: { height: "100%", overflow: "visible" }, onBeforeInputCapture: preventReadonlyInput, onCutCapture: preventReadonlyInput, onPasteCapture: preventReadonlyInput, onDropCapture: preventReadonlyInput, onKeyDownCapture: preventReadonlyKey, children: _jsx(Milkdown, {}) }));
}
export function MilkdownEditor({ pageId, defaultValue, isEditable, bridge }) {
    return (_jsx(MilkdownProvider, { children: _jsx(EditorSurface, { pageId: pageId, defaultValue: defaultValue, isEditable: isEditable, bridge: bridge }) }));
}
