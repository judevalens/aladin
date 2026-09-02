package docsurface

import (
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

// A fixture that stamps two anchors as ordinary DOM metadata and rejects a promise
// with no catch — the failure mode CDP's exceptionThrown does NOT report.
const fixtureAnchorBundleJS = `(function(){
  var r = document.getElementById('root');
  var intro = document.createElement('div');
  intro.setAttribute('data-anchor', 'intro');
  intro.textContent = 'intro region';
  r.appendChild(intro);
  var table = document.createElement('div');
  table.setAttribute('data-anchor', 'positions:table');
  table.textContent = 'table region';
  r.appendChild(table);
  Promise.reject(new Error('load failed'));
})();`

// The two verification checks that did not exist before, proven in a real
// renderer: anchors are counted from the live DOM, and an unhandled rejection
// is captured (as a console error) instead of vanishing.
func TestPreviewSession_AnchorsAndRejections(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})
	if err := m.store.WriteFile(ctx, "p1", "dist/bundle.js", []byte(fixtureAnchorBundleJS)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := m.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	counts, err := m.CheckAnchors(ctx, "p1", []string{"intro", "positions:table", "never-rendered"})
	if err != nil {
		t.Fatalf("CheckAnchors: %v", err)
	}
	if counts["intro"] != 1 {
		t.Errorf("intro count = %d, want 1", counts["intro"])
	}
	// An id containing ':' must survive selector escaping.
	if counts["positions:table"] != 1 {
		t.Errorf("positions:table count = %d, want 1", counts["positions:table"])
	}
	if counts["never-rendered"] != 0 {
		t.Errorf("never-rendered count = %d, want 0", counts["never-rendered"])
	}

	// The rejection lands asynchronously; poll briefly.
	deadline := time.Now().Add(3 * time.Second)
	var errs []string
	for time.Now().Before(deadline) {
		errs, err = m.ConsoleErrors(ctx, "p1")
		if err != nil {
			t.Fatalf("ConsoleErrors: %v", err)
		}
		if len(errs) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), "unhandledrejection") {
		t.Fatalf("unhandled rejection not captured, console errors = %v", errs)
	}
}
