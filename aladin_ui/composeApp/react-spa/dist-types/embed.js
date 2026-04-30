import { jsx as _jsx } from "react/jsx-runtime";
import { createRoot } from "react-dom/client";
import App from "./App";
import cssText from "./embed.css?inline";
const roots = new WeakMap();
const overlayRoots = new WeakMap();
function ensureStyles(host) {
    const rootNode = host.getRootNode();
    const selector = 'style[data-aladin-artifact-spa="true"]';
    if (rootNode instanceof ShadowRoot) {
        if (!rootNode.querySelector(selector)) {
            const style = document.createElement("style");
            style.setAttribute("data-aladin-artifact-spa", "true");
            style.textContent = cssText;
            rootNode.appendChild(style);
        }
        return;
    }
    if (!document.head.querySelector(selector)) {
        const style = document.createElement("style");
        style.setAttribute("data-aladin-artifact-spa", "true");
        style.textContent = cssText;
        document.head.appendChild(style);
    }
}
function ensureOverlayRoot(host) {
    const rootNode = host.getRootNode();
    const selector = '[data-aladin-artifact-overlay-root="true"]';
    if (rootNode instanceof ShadowRoot) {
        const existing = rootNode.querySelector(selector);
        if (existing) {
            overlayRoots.set(host, existing);
            return existing;
        }
        const overlayRoot = document.createElement("div");
        overlayRoot.setAttribute("data-aladin-artifact-overlay-root", "true");
        overlayRoot.style.position = "fixed";
        overlayRoot.style.inset = "0";
        overlayRoot.style.pointerEvents = "none";
        overlayRoot.style.zIndex = "2147483647";
        rootNode.appendChild(overlayRoot);
        overlayRoots.set(host, overlayRoot);
        return overlayRoot;
    }
    let existing = document.body.querySelector(selector);
    if (!existing) {
        existing = document.createElement("div");
        existing.setAttribute("data-aladin-artifact-overlay-root", "true");
        existing.style.position = "fixed";
        existing.style.inset = "0";
        existing.style.pointerEvents = "none";
        existing.style.zIndex = "2147483647";
        document.body.appendChild(existing);
    }
    overlayRoots.set(host, existing);
    return existing;
}
function mount(element, bridge, title) {
    ensureStyles(element);
    const overlayRoot = ensureOverlayRoot(element);
    const root = roots.get(element) ?? createRoot(element);
    roots.set(element, root);
    root.render(_jsx(App, { widgetId: title, overlayRoot: overlayRoot, bridge: bridge }));
}
function unmount(element) {
    const root = roots.get(element);
    if (root) {
        root.unmount();
        roots.delete(element);
    }
    overlayRoots.delete(element);
}
function createBridge(jsEventHandler) {
    let bridge = {
        mount: (root) => mount(root, bridge),
        unmount: unmount,
        kotlinEvent: (event) => {
            console.log("Kotlin event received: ", event);
        },
        jsEvent: (event) => {
            console.log("JS event received: ", event);
            jsEventHandler(event);
        },
    };
    return bridge;
}
window.createBridge = createBridge;
window.AladinArtifactSpa = {
    mount,
    unmount,
};
