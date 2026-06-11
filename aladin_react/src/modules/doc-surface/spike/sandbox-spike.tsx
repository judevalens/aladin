import { useEffect, useRef, useState } from "react";

/**
 * Phase 0 — Doc Surface security spike.
 *
 * Proves that a `sandbox="allow-scripts"` iframe (no `allow-same-origin`) with a
 * meta-CSP isolates untrusted agent code inside the actual Tauri webview:
 *   - cannot reach Aladin's DOM (parent/top document)
 *   - cannot read Aladin cookies / localStorage
 *   - cannot fetch off-origin or same-origin (CSP connect-src 'none')
 *   - CAN do exactly one thing: a postMessage bridge round-trip (ping -> pong)
 *
 * The iframe is deliberately HOSTILE: it tries every escape, catches the result,
 * and reports back over postMessage (the one channel that is supposed to work).
 * Load at /spike/sandbox in the browser dev server AND in `npm run tauri:dev`.
 */

type TestResult = { name: string; passed: boolean; detail: string };

// The single allowed bridge command. Everything else is rejected by the host.
const ALLOWED_CMD = "ping";

function buildHostileSrcdoc(): string {
  // NOTE: the CSP meta must be first in <head>. connect-src 'none' is what blocks
  // exfiltration; script-src 'unsafe-inline' only lets OUR inline test script run —
  // safe because the frame has an opaque origin and can reach nothing.
  return `<!doctype html>
<html>
<head>
<meta http-equiv="Content-Security-Policy"
  content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'none'; img-src data:;">
<style>body{font:13px monospace;margin:8px;color:#bbb;background:#111}</style>
</head>
<body>
<div id="status">hostile frame running escape attempts…</div>
<script>
(async function () {
  var results = [];
  function record(name, passed, detail) { results.push({ name: name, passed: passed, detail: String(detail) }); }

  // 1. Read Aladin's DOM via parent
  try {
    var html = window.parent.document.body.innerHTML;
    record("read parent.document", false, "ESCAPED: read " + html.length + " chars of host DOM");
  } catch (e) { record("read parent.document", true, "blocked: " + e.name); }

  // 2. Read top window location
  try {
    var href = window.top.location.href;
    record("read top.location.href", false, "ESCAPED: " + href);
  } catch (e) { record("read top.location.href", true, "blocked: " + e.name); }

  // 3. Read cookies (should be empty: opaque origin has no host cookies)
  try {
    var c = document.cookie;
    record("read document.cookie", c === "", c === "" ? "empty (no host cookies)" : "GOT COOKIES: " + c);
  } catch (e) { record("read document.cookie", true, "blocked: " + e.name); }

  // 4. Access own localStorage (opaque origin -> should throw)
  try {
    var ls = window.localStorage; void ls.length;
    record("access localStorage", false, "ESCAPED: localStorage usable");
  } catch (e) { record("access localStorage", true, "blocked: " + e.name); }

  // 5. Access parent's localStorage
  try {
    var pls = window.parent.localStorage; void pls.length;
    record("access parent.localStorage", false, "ESCAPED: read host storage");
  } catch (e) { record("access parent.localStorage", true, "blocked: " + e.name); }

  // 6. Exfiltrate: off-origin fetch (CSP connect-src 'none' must block)
  try {
    await fetch("https://example.com/", { mode: "no-cors" });
    record("off-origin fetch", false, "ESCAPED: exfiltration succeeded");
  } catch (e) { record("off-origin fetch", true, "blocked: " + e.name); }

  // 7. Same-origin fetch (relative URL -> also must be blocked by CSP)
  try {
    await fetch("/", {});
    record("same-origin fetch /", false, "ESCAPED: reached host origin");
  } catch (e) { record("same-origin fetch /", true, "blocked: " + e.name); }

  document.getElementById("status").textContent = "escape attempts done; testing bridge…";

  // 8. The ONE allowed channel: postMessage bridge round-trip.
  var bridgeDone = false;
  function finish() {
    document.getElementById("status").textContent = "done — reported " + results.length + " results";
    window.parent.postMessage({ type: "spike-report", results: results }, "*");
  }
  window.addEventListener("message", function (e) {
    if (e.data && e.data.type === "bridge-reply" && e.data.id === 42) {
      bridgeDone = true;
      var ok = e.data.result === "pong";
      record("bridge ping -> pong", ok, ok ? "received pong over postMessage" : "bad reply: " + e.data.result);
      finish();
    }
  });
  window.parent.postMessage({ type: "bridge", cmd: "${ALLOWED_CMD}", id: 42 }, "*");
  setTimeout(function () {
    if (!bridgeDone) { record("bridge ping -> pong", false, "TIMEOUT: no reply from host"); finish(); }
  }, 1500);
})();
</script>
</body>
</html>`;
}

export function SandboxSpike() {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [results, setResults] = useState<TestResult[] | null>(null);
  const [bridgeCalls, setBridgeCalls] = useState<string[]>([]);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    function onMessage(e: MessageEvent) {
      // Origin check: a srcdoc sandboxed frame has an OPAQUE origin, so event.origin
      // is "null" and cannot be trusted. The reliable check is the source window.
      if (e.source !== iframeRef.current?.contentWindow) return;
      const data = e.data as { type?: string; cmd?: string; id?: number; results?: TestResult[] };

      if (data?.type === "bridge") {
        // Host enforces scope: only ALLOWED_CMD is honored.
        if (data.cmd === ALLOWED_CMD) {
          setBridgeCalls((c) => [...c, `accepted "${data.cmd}" (id ${data.id}) -> pong`]);
          (e.source as Window).postMessage({ type: "bridge-reply", id: data.id, result: "pong" }, "*");
        } else {
          setBridgeCalls((c) => [...c, `REJECTED out-of-scope cmd "${data.cmd}"`]);
        }
      } else if (data?.type === "spike-report") {
        const rs = data.results ?? [];
        setResults(rs);
        // Forwarded to the Tauri (wry) stdout in dev, so the WKWebView verdict
        // is readable from the dev log without driving the desktop UI.
        const allOk = rs.length > 0 && rs.every((r) => r.passed);
        console.log(`[SPIKE] verdict=${allOk ? "ISOLATION_HOLDS" : "ISOLATION_BROKEN"} (${rs.length} checks)`);
        rs.forEach((r) => console.log(`[SPIKE] ${r.passed ? "PASS" : "FAIL"} ${r.name} :: ${r.detail}`));
      }
    }
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  const allPassed = results !== null && results.every((r) => r.passed);

  return (
    <div className="min-h-screen bg-bg p-6 text-ink font-mono text-sm">
      <h1 className="mb-1 font-display text-lg text-ink">Phase 0 — Sandbox isolation spike</h1>
      <p className="mb-4 text-ink-3">
        Hostile <code>sandbox=&quot;allow-scripts&quot;</code> iframe (no <code>allow-same-origin</code>) +
        meta-CSP <code>connect-src &apos;none&apos;</code>. Every row should be BLOCKED; the bridge should round-trip.
      </p>

      <div className="mb-4 flex items-center gap-3">
        <button
          className="rounded-chip border border-line bg-field px-3 py-1 text-ink hover:bg-raise"
          onClick={() => {
            setResults(null);
            setBridgeCalls([]);
            setNonce((n) => n + 1);
          }}
        >
          Re-run
        </button>
        {results !== null && (
          <span className={allPassed ? "text-for" : "text-against"}>
            {allPassed ? "✓ ISOLATION HOLDS — all escapes blocked, bridge works" : "✗ ISOLATION BROKEN — see failures below"}
          </span>
        )}
      </div>

      <div className="grid grid-cols-2 gap-6">
        <div>
          <div className="mb-2 text-ink-2">Results</div>
          {results === null ? (
            <div className="text-ink-3">running…</div>
          ) : (
            <ul className="space-y-1">
              {results.map((r, i) => (
                <li key={i} className="flex gap-2">
                  <span className={r.passed ? "text-for" : "text-against"}>{r.passed ? "PASS" : "FAIL"}</span>
                  <span className="text-ink">{r.name}</span>
                  <span className="text-ink-3">— {r.detail}</span>
                </li>
              ))}
            </ul>
          )}
          <div className="mt-4 mb-2 text-ink-2">Host bridge log</div>
          <ul className="space-y-1 text-ink-3">
            {bridgeCalls.length === 0 ? <li>(none yet)</li> : bridgeCalls.map((c, i) => <li key={i}>{c}</li>)}
          </ul>
        </div>

        <div>
          <div className="mb-2 text-ink-2">The hostile frame</div>
          <iframe
            key={nonce}
            ref={iframeRef}
            title="hostile-sandbox"
            sandbox="allow-scripts"
            srcDoc={buildHostileSrcdoc()}
            className="h-64 w-full rounded-card border border-line bg-panel"
          />
        </div>
      </div>
    </div>
  );
}
