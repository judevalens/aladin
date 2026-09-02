package docsurface

import (
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

// A fixture bundle that speaks the kv bridge protocol directly (no SDK import —
// same style as fixtureBundleJS): set → get → render, then a deliberately stale
// set to prove the conflict reply shape. Rendered markers are asserted below.
const fixtureKVBundleJS = `(function(){
  var r = document.getElementById('root');
  var seq = 0, pending = {};
  window.addEventListener('message', function(e){
    var m = e.data;
    if (!m || m.aladin !== 'bridge/1' || m.type !== 'response') return;
    var p = pending[m.id]; delete pending[m.id];
    if (p) p(m);
  });
  function call(method, params){
    return new Promise(function(resolve){
      var id = ++seq; pending[id] = resolve;
      window.postMessage({aladin:'bridge/1', type:'request', id:id, method:method, params:params}, '*');
    });
  }
  function mark(text){
    var div = document.createElement('div');
    div.textContent = text;
    r.appendChild(div);
  }
  mark('booted');
  call('kv.set', {key:'timer/target', value:{count:5}, baseRevision:0})
    .then(function(res){
      mark('set:' + (res.ok ? res.data.revision : 'FAIL'));
      return call('kv.get', {key:'timer/target'});
    })
    .then(function(res){
      mark('get:' + (res.ok && res.data ? res.data.value.count : 'FAIL'));
      // Stale write: baseRevision 0 against stored revision 1 → conflict.
      return call('kv.set', {key:'timer/target', value:{count:9}, baseRevision:0});
    })
    .then(function(res){
      var c = (!res.ok && res.code === 'conflict') ? res.data.currentRevision : 'FAIL';
      mark('conflict:' + c);
    });
})();`

// End-to-end shard-KV proof in a real renderer: the preview emulator honors the
// revision guard and the conflict reply carries the current revision — so a
// stateful shard (useShardState) runs headless with real concurrency semantics.
func TestPreviewSession_KVEmulatorRoundTrip(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})
	if err := m.store.WriteFile(ctx, "p1", "dist/bundle.js", []byte(fixtureKVBundleJS)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := m.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	// The async round-trips settle in a few frames; poll the outline briefly.
	deadline := time.Now().Add(3 * time.Second)
	var outline string
	for time.Now().Before(deadline) {
		snap, err := m.Snapshot(ctx, "p1")
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		outline = snap.Outline
		if strings.Contains(outline, "conflict:") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, want := range []string{"set:1", "get:5", "conflict:1"} {
		if !strings.Contains(outline, want) {
			t.Errorf("outline missing %q:\n%s", want, outline)
		}
	}
}
