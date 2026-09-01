package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/evanw/esbuild/pkg/api"
)

// Compile the real frontend host modules into a minimal test shell. The iframe
// loads protected HTTP content with a content-only token; only the shell knows
// the session credential. No CDP resource dispatcher participates in this path.
func exerciseShardWebHost(t *testing.T, ctx context.Context, backend http.Handler, shardID string, mutate func()) {
	t.Helper()
	frontend, err := filepath.Abs("../../../aladin_react")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := json.Marshal(shardID)
	script := `import {createApiClient} from "./src/shared/api/client";
import {createResourceHostHub} from "./src/modules/doc-surface/bridge/resource-host-hub";
import {createBridgeV2Host} from "./src/modules/doc-surface/bridge/bridge-v2-host";
let credential="pilot-test-token";
const pilot=window.pilot={views:{},sockets:0,error:"",revoke:()=>{credential="expired";}};
const api=createApiClient({apiBaseUrl:""},{getToken:()=>credential});
const hub=createResourceHostHub(api,location.origin.replace("http","ws")+"/api/shard-resources/ws",()=>credential,url=>{
 const socket=new WebSocket(url);pilot.sockets++;pilot.disconnect=()=>socket.close();return socket;
});
(async()=>{
 const shardId=` + string(id) + `,environment="published";
 const release=await hub.release(shardId,environment);
 for(let i=0;i<2;i++){
  const frame=document.createElement("iframe");frame.sandbox="allow-scripts";
  const host=createBridgeV2Host({target:{shardId,environment,contractHash:release.contractHash},buildId:release.buildId,getWindow:()=>frame.contentWindow,getTheme:()=>"light",hub});
  window.addEventListener("message",event=>{if(event.source===frame.contentWindow&&event.data?.type==="pilot.render")pilot.views[i]=event.data.text;});
  document.body.append(frame);host.attach();
  frame.src="/content/"+shardId+"/?access_token=pilot-content-only&build_id="+release.buildId;
 }
})().catch(error=>{pilot.error=String(error?.message||error);});`
	build := api.Build(api.BuildOptions{AbsWorkingDir: frontend, Stdin: &api.StdinOptions{Contents: script, ResolveDir: frontend, Loader: api.LoaderTS}, Alias: map[string]string{"@": filepath.Join(frontend, "src")}, Bundle: true, Write: false, Format: api.FormatIIFE, Platform: api.PlatformBrowser, Target: api.ES2022})
	if len(build.Errors) > 0 {
		t.Fatalf("host test bundle: %+v", build.Errors)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pilot":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!doctype html><html><body><script src="/pilot.js"></script></body></html>`))
		case "/pilot.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write(build.OutputFiles[0].Contents)
		default:
			backend.ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.NoSandbox, chromedp.DisableGPU)
	if executable := os.Getenv("DOCSURFACE_CHROME_PATH"); executable != "" {
		opts = append(opts, chromedp.ExecPath(executable))
	}
	allocator, closeAllocator := chromedp.NewExecAllocator(ctx, opts...)
	defer closeAllocator()
	browser, closeBrowser := chromedp.NewContext(allocator)
	defer closeBrowser()
	if err := chromedp.Run(browser, chromedp.Navigate(server.URL+"/pilot")); err != nil {
		t.Fatal(err)
	}
	wait := func(expression string) {
		t.Helper()
		until := time.Now().Add(10 * time.Second)
		for time.Now().Before(until) {
			var ready bool
			if err := chromedp.Run(browser, chromedp.Evaluate(expression, &ready)); err != nil {
				t.Fatal(err)
			}
			if ready {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		var state string
		_ = chromedp.Run(browser, chromedp.Evaluate("JSON.stringify(window.pilot)", &state))
		t.Fatalf("web host did not satisfy %s: %s", expression, state)
	}
	wait(`Object.keys(window.pilot.views).length===2 && Object.values(window.pilot.views).every(text=>text.includes("Written with iframe closed"))`)
	var sockets int
	if err := chromedp.Run(browser, chromedp.Evaluate("window.pilot.sockets", &sockets)); err != nil || sockets != 1 {
		t.Fatalf("frames did not share one socket: %d %v", sockets, err)
	}
	mutate()
	wait(`Object.values(window.pilot.views).every(text=>text.includes("Agent change in both web frames"))`)
	start := time.Now()
	if err := chromedp.Run(browser, chromedp.Evaluate("window.pilot.disconnect()", nil)); err != nil {
		t.Fatal(err)
	}
	wait(`window.pilot.sockets===2 && Object.values(window.pilot.views).every(text=>text.startsWith("live:")&&text.includes("Agent change in both web frames"))`)
	t.Logf("shared host reconnect + restored views: %s", time.Since(start))
	if err := chromedp.Run(browser, chromedp.Evaluate("window.pilot.revoke()", nil)); err != nil {
		t.Fatal(err)
	}
	wait(`Object.values(window.pilot.views).every(text=>text==="forbidden:[]")`)
}
