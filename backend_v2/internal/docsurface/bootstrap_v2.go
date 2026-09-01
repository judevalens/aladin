package docsurface

import (
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
)

// Inject only data pinned to the same immutable build. JSON escaping prevents
// authored strings from breaking out of the inert bootstrap script element.
func BootstrapV2(html string, release service.ResourceRelease) string {
	var contract shardv2.Contract
	if json.Unmarshal(release.Source, &contract) != nil {
		return html
	}
	raw, _ := json.Marshal(map[string]any{"protocol": "bridge/2", "buildId": release.BuildID, "contractHash": release.Hash, "bindings": contract.Bindings})
	return strings.Replace(html, "<head>", "<head><script id=\"aladin-resource-bootstrap\" type=\"application/json\">"+string(raw)+"</script>", 1)
}
