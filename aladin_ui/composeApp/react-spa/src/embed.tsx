import React from "react";
import {createRoot, Root} from "react-dom/client";
import App from "./App";
import cssText from "./embed.css?inline";

type ArtifactMountPayload = {
    title?: string;
    kind?: string;
};

const roots = new WeakMap<HTMLElement, Root>();
const overlayRoots = new WeakMap<HTMLElement, HTMLElement>();

function ensureStyles(host: HTMLElement) {
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

function ensureOverlayRoot(host: HTMLElement): HTMLElement {
    const rootNode = host.getRootNode();
    const selector = '[data-aladin-artifact-overlay-root="true"]';

    if (rootNode instanceof ShadowRoot) {
        const existing = rootNode.querySelector<HTMLElement>(selector);
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

    let existing = document.body.querySelector<HTMLElement>(selector);
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

function mount(
    element: HTMLElement,
    bridge: Bridge,
    title?: string,
) {
    ensureStyles(element);
    const overlayRoot = ensureOverlayRoot(element);
    const root = roots.get(element) ?? createRoot(element);
    roots.set(element, root);
    root.render(
        <App widgetId={title} overlayRoot={overlayRoot} bridge={bridge}/>,
    );
}

function unmount(element: HTMLElement) {
    const root = roots.get(element);
    if (root) {
        root.unmount();
        roots.delete(element);
    }
    overlayRoots.delete(element);
}

type Event = {
    type: string;
    payload: Object;
};

export type Bridge = {
    mount: (root: HTMLElement) => void;
    unmount: (root: HTMLElement) => void;
    kotlinEvent: (event: Event) => void;
    jsEvent: (event: Event) => void;
}

function createBridge(jsEventHandler: (event: Event) => void): Bridge {
    let bridge = {
        mount: (root: HTMLElement) => mount(root, bridge),
        unmount: unmount,
        kotlinEvent: (event: Event) => {
            console.log("Kotlin event received: ", event);
        },
        jsEvent: (event: Event) => {
            console.log("JS event received: ", event);
            jsEventHandler(event);
        },
    };
    return bridge;
}

declare global {
    interface Window {
        AladinArtifactSpa: {
            mount: typeof mount;
            unmount: typeof unmount;
        };
        createBridge: typeof createBridge;

    }
}
window.createBridge = createBridge
window.AladinArtifactSpa = {
    mount,
    unmount,
};
