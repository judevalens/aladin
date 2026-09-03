package docsurface

import (
	"encoding/json"
	"strings"
)

func normalizeRoute(r string) string {
	r = strings.TrimSpace(r)
	if r == "" {
		return "#/"
	}
	if strings.HasPrefix(r, "#") {
		return r
	}
	if strings.HasPrefix(r, "/") {
		return "#" + r
	}
	return "#/" + r
}

// wrapEval makes any agent expression return a JSON string (or "undefined" /
// "error: ...") so the result is always a readable scalar for the agent.
func wrapEval(expr string) string {
	return "(function(){try{var v=(" + expr +
		");return (typeof v==='undefined')?'undefined':JSON.stringify(v);}" +
		"catch(e){return 'error: '+((e&&e.message)||String(e));}})()"
}

// jsString renders s as a safely-quoted JS string literal.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
