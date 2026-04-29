import {createPortal} from "react-dom";
import {useState} from "react";

export type ArtifactSpaProps = {
    title?: string,
    kind?: string,
    overlayRoot?: HTMLElement | null,
};

export default function App({
                                title = "Artifact web view prototype",
                                kind = "artifact",
                                overlayRoot = null,
                            }: ArtifactSpaProps) {
    const [showOverlay, setShowOverlay] = useState(false);

    return (
        <>
            <main className="app-shell">
                <article className="document-page">
                    <header className="document-header">
                        <div className="document-meta-row">
                            <span>{kind}</span>
                            <span>local draft</span>
                        </div>
                        <div className="document-title-row">
                            <h1>{title}</h1>
                            <button
                                className="overlay-toggle"
                                type="button"
                                onClick={() => setShowOverlay((value) => !value)}
                            >
                                {showOverlay ? "Hide probe" : "Probe"}
                            </button>
                        </div>
                    </header>

                    <section className="document-body">
                        <p className="document-lead">
                            This surface is deliberately quiet. It should read like the beginning of a
                            writing canvas, with just enough context to orient the document before the
                            editor takes over.
                        </p>

                        <p>
                            Pane 3 now behaves like a document window: tabs at the top, a compact
                            internal header, and the working surface starting almost immediately below
                            it. The next step is to replace this stub with the actual markdown editor.
                        </p>

                        <p>
                            Graph mode can reuse the same bounded-canvas model later, keeping local
                            overlays and interactions inside the pane instead of letting the shell
                            dominate the experience.
                        </p>
                        <p>
                            Graph mode can reuse the same bounded-canvas model later, keeping local
                            overlays and interactions inside the pane instead of letting the shell
                            dominate the experience.
                        </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>   <p>
                        Graph mode can reuse the same bounded-canvas model later, keeping local
                        overlays and interactions inside the pane instead of letting the shell
                        dominate the experience.
                    </p>
                    </section>
                </article>
            </main>

            {showOverlay &&
                overlayRoot &&
                createPortal(
                    <div
                        className="boundary-overlay-shell"
                        onClick={() => setShowOverlay(false)}
                    >
                        <div
                            className="boundary-overlay-card"
                            onClick={(event) => event.stopPropagation()}
                        >
                            <div className="panel-label">REACT PORTAL OVERLAY</div>
                            <h2>Cross-boundary overlay probe</h2>
                            <p>
                                This overlay is rendered by React outside the embedded artifact
                                root. If you can see it covering the folder browser or top bar,
                                the React side can own overlays above Compose surfaces.
                            </p>
                            <button
                                className="overlay-toggle"
                                type="button"
                                onClick={() => setShowOverlay(false)}
                            >
                                Close overlay
                            </button>
                        </div>
                    </div>,
                    overlayRoot,
                )}
        </>
    );
}
