import { useEffect, useRef, useState } from "react";

/**
 * Local-vendor-cache spike.
 *
 * Proves the ONE unknown for serving vendored deps from a local Tauri cache:
 * can a `sandbox="allow-scripts"` iframe (opaque origin) with a meta-CSP load an
 * ES MODULE through a CUSTOM Tauri URI scheme (`vendor://`) — i.e. via an import
 * map `{ "testdep": "vendor://deps/<sha>" }` + `<script type="module">`?
 *
 * Custom schemes + sandbox + module imports behave differently in WKWebView than
 * in Chrome, so this MUST be run in the real desktop app:
 *
 *     npm run tauri:dev    →  navigate to  /spike/vendor-scheme
 *
 * In a plain browser dev server the `vendor://` scheme is NOT registered, so the
 * import fails — that's EXPECTED. A PASS is only meaningful inside tauri:dev.
 * The verdict is shown on screen AND console.log'd to the wry stdout.
 */

const SCHEME_URL = "vendor://deps/spike-sha-0000000000000000000000000000000000000000000000000000000000000000";

function buildSrcdoc(): string {
  // The import map must precede the module script. script-src allows the custom
  // `vendor:` scheme (+ 'unsafe-inline' for OUR inline test module). connect-src
  // 'none' is kept to match production — module loads ride script-src, not connect.
  return `<!doctype html>
<html>
<head>
<meta http-equiv="Content-Security-Policy"
  content="default-src 'none'; script-src 'unsafe-inline' vendor:; style-src 'unsafe-inline'; connect-src 'none'; img-src data:;">
<style>body{font:13px monospace;margin:8px;color:#bbb;background:#111}</style>
<script type="importmap">{"imports":{"testdep":"${SCHEME_URL}"}}</script>
</head>
<body>
<div id="s">loading a vendor:// ES module…</div>
<script type="module">
(async () => {
  function report(ok, detail) {
    document.getElementById("s").textContent = (ok ? "OK · " : "FAIL · ") + detail;
    window.parent.postMessage({ type: "vendor-spike", ok: ok, detail: String(detail) }, "*");
  }
  try {
    const m = await import("testdep");                 // resolves via import map -> vendor://
    const ok = !!(m && typeof m.ping === "function" && m.ping() === "vendor-scheme-ok" && m.ANSWER === 42);
    report(ok, ok ? "imported vendor:// module → " + m.ping() + " (ANSWER=" + m.ANSWER + ")"
                  : "module loaded but exports were unexpected");
  } catch (e) {
    report(false, "could not load vendor:// module: " + (e && e.message ? e.message : e));
  }
})();
</script>
</body>
</html>`;
}

export function VendorSchemeSpike() {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [result, setResult] = useState<{ ok: boolean; detail: string } | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    function onMessage(e: MessageEvent) {
      // Sandboxed srcdoc frame has an opaque origin → trust the source window, not origin.
      if (e.source !== iframeRef.current?.contentWindow) return;
      const data = e.data as { type?: string; ok?: boolean; detail?: string };
      if (data?.type === "vendor-spike") {
        const r = { ok: !!data.ok, detail: data.detail ?? "" };
        setResult(r);
        console.log(`[VENDOR-SPIKE] verdict=${r.ok ? "SCHEME_LOADS" : "SCHEME_BLOCKED"} :: ${r.detail}`);
      }
    }
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  return (
    <div className="min-h-screen bg-bg p-6 text-ink font-mono text-sm">
      <h1 className="mb-1 font-display text-lg text-ink">Local-vendor-cache spike — custom scheme in sandbox</h1>
      <p className="mb-1 text-ink-3">
        A <code>sandbox=&quot;allow-scripts&quot;</code> iframe imports an ES module via an import map →{" "}
        <code>vendor://</code> (a custom Tauri scheme served from Rust).
      </p>
      <p className="mb-4 text-ink-4">
        Run in <code>npm run tauri:dev</code> → <code>/spike/vendor-scheme</code>. PASS here means a local cache can
        serve deps to the sandbox with zero network. (In a plain browser the scheme isn&apos;t registered → expected FAIL.)
      </p>

      <div className="mb-4 flex items-center gap-3">
        <button
          className="rounded-chip border border-line bg-field px-3 py-1 text-ink hover:bg-raise"
          onClick={() => { setResult(null); setNonce((n) => n + 1); }}
        >
          Re-run
        </button>
        {result && (
          <span className={result.ok ? "text-for" : "text-against"}>
            {result.ok ? "✓ SCHEME LOADS — vendor:// module imported inside the sandbox" : "✗ SCHEME BLOCKED — see detail"}
          </span>
        )}
      </div>

      <div className="grid grid-cols-2 gap-6">
        <div>
          <div className="mb-2 text-ink-2">Verdict</div>
          {result === null ? (
            <div className="text-ink-3">running…</div>
          ) : (
            <div className="flex gap-2">
              <span className={result.ok ? "text-for" : "text-against"}>{result.ok ? "PASS" : "FAIL"}</span>
              <span className="text-ink-3">{result.detail}</span>
            </div>
          )}
          <div className="mt-4 text-ink-4">
            scheme URL tested:<br />
            <span className="text-ink-3">{SCHEME_URL}</span>
          </div>
        </div>
        <div>
          <div className="mb-2 text-ink-2">The sandboxed frame</div>
          <iframe
            key={nonce}
            ref={iframeRef}
            title="vendor-scheme-sandbox"
            sandbox="allow-scripts"
            srcDoc={buildSrcdoc()}
            className="h-48 w-full rounded-card border border-line bg-panel"
          />
        </div>
      </div>
    </div>
  );
}
