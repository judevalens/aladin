import { jsx as _jsx } from "react/jsx-runtime";
import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./styles.css";
import cssText from "./styles.css?inline";
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
function mount(element, title, kind) {
    ensureStyles(element);
    const overlayRoot = ensureOverlayRoot(element);
    const root = roots.get(element) ?? createRoot(element);
    roots.set(element, root);
    root.render(_jsx(React.StrictMode, { children: _jsx(App, { title: title, kind: kind, overlayRoot: overlayRoot }) }));
}
function unmount(element) {
    const root = roots.get(element);
    if (root) {
        root.unmount();
        roots.delete(element);
    }
    overlayRoots.delete(element);
}
window.AladinArtifactSpa = {
    mount,
    unmount,
};
