package docsurface

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/service"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func (m *PreviewSessions) captureState(opCtx context.Context, s *previewSession, pageID string) service.PreviewState {
	var snap struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Mounted bool   `json:"mounted"`
	}
	_ = chromedp.Run(opCtx, chromedp.Evaluate(stateJS, &snap))
	st := service.PreviewState{
		PageID:  pageID,
		URL:     snap.URL,
		Title:   snap.Title,
		Mounted: snap.Mounted,
	}
	st.Console, st.Exceptions = s.drainLogs()
	return st
}

// --- session event capture -------------------------------------------------

func (s *previewSession) onEvent(ev any) {
	switch e := ev.(type) {
	case *cdpruntime.EventBindingCalled:
		if e.Name != "aladinPreviewResource" {
			return
		}
		s.resourceMu.Lock()
		if s.resourceQueue != nil {
			select {
			case s.resourceQueue <- e.Payload:
			default:
				if s.resourceCancel != nil {
					s.resourceCancel()
				}
			}
		}
		s.resourceMu.Unlock()
	case *cdpruntime.EventConsoleAPICalled:
		line := string(e.Type) + ": " + formatConsoleArgs(e.Args)
		s.logMu.Lock()
		s.console = appendCapped(s.console, line, maxLogLines)
		// console.error is partitioned out as well as kept in the full log: the
		// verify pass reports it separately (and can be told to fail on it),
		// while `console` stays the complete transcript for debugging.
		if e.Type == "error" {
			s.consoleErrors = appendCapped(s.consoleErrors, line, maxLogLines)
		}
		s.logMu.Unlock()
	case *cdpruntime.EventExceptionThrown:
		msg := e.ExceptionDetails.Text
		if ex := e.ExceptionDetails.Exception; ex != nil && ex.Description != "" {
			msg = ex.Description
		}
		s.logMu.Lock()
		s.exceptions = appendCapped(s.exceptions, msg, maxLogLines)
		s.logMu.Unlock()
	}
}

func (s *previewSession) resetLogs() {
	s.logMu.Lock()
	s.console = nil
	s.consoleErrors = nil
	s.exceptions = nil
	s.logMu.Unlock()
}

func (s *previewSession) drainLogs() (console, exceptions []string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return lastN(s.console, logReturn), lastN(s.exceptions, logReturn)
}

// drainConsoleErrors returns the console.error lines accumulated since the last
// reset (the verify pass reads these separately from the full console).
func (s *previewSession) drainConsoleErrors() []string {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return lastN(s.consoleErrors, logReturn)
}

// --- helpers ---------------------------------------------------------------

// runFirst runs the tab's first chromedp action set on the RAW tab context (no
// timeout — a timeout on the first Run stops the whole browser per chromedp).
// A watchdog bounds it: if the browser hangs starting, the tab is cancelled and
// an error returned instead of blocking forever.

func formatConsoleArgs(args []*cdpruntime.RemoteObject) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, formatConsoleArg(a))
	}
	return strings.Join(parts, " ")
}

func formatConsoleArg(a *cdpruntime.RemoteObject) string {
	if a == nil {
		return ""
	}
	if len(a.Value) > 0 {
		var v any
		if err := json.Unmarshal(a.Value, &v); err == nil {
			return fmt.Sprintf("%v", v)
		}
		return string(a.Value)
	}
	if a.Description != "" {
		return a.Description
	}
	return string(a.Type)
}

func appendCapped(s []string, v string, max int) []string {
	s = append(s, v)
	if len(s) > max {
		s = s[len(s)-max:]
	}
	return s
}

func lastN(s []string, n int) []string {
	if len(s) == 0 {
		return nil
	}
	if len(s) <= n {
		out := make([]string, len(s))
		copy(out, s)
		return out
	}
	out := make([]string, n)
	copy(out, s[len(s)-n:])
	return out
}

// --- JS snippets -----------------------------------------------------------

const stateJS = `({url:location.href,title:document.title,` +
	`mounted:!!document.getElementById('root')&&document.getElementById('root').childElementCount>0})`

// snapshotJS returns the current view's innerText (capped) plus a compact,
// depth-limited element outline (tag#id.class "own text").
const snapshotJS = `(function(){
  function walk(el,depth,lines){
    if(depth>6||lines.length>=200) return;
    for(var i=0;i<el.children.length;i++){
      if(lines.length>=200) return;
      var c=el.children[i];
      var s=c.tagName.toLowerCase();
      if(c.id) s+='#'+c.id;
      if(typeof c.className==='string'&&c.className.trim()) s+='.'+c.className.trim().split(/\s+/).slice(0,3).join('.');
      var own='';
      for(var j=0;j<c.childNodes.length;j++){var n=c.childNodes[j];if(n.nodeType===3){own+=' '+n.textContent.trim();}}
      own=own.trim();
      if(own) s+=' "'+own.slice(0,60)+'"';
      lines.push(new Array(depth+1).join('  ')+s);
      walk(c,depth+1,lines);
    }
  }
  var lines=[];walk(document.body,0,lines);
  var text=(document.body.innerText||'').slice(0,2000);
  return 'TEXT:\n'+text+'\n\nDOM:\n'+lines.join('\n');
})()`

// setDocContent replaces the tab's document with html (the frame id comes from
// the live frame tree, which requires a prior navigation to establish it).
