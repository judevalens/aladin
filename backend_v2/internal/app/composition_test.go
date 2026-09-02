package app_test

import (
	"reflect"
	"testing"

	"aladin/backend_v2/internal/api"
	"aladin/backend_v2/internal/app"
	mcpserver "aladin/backend_v2/internal/mcp"
)

// These assignments make process boundaries compile-time contracts. A factory
// cannot drop a required consumer service, and the consumers do not depend on a
// shared application-wide interface.
var (
	_ api.Dependencies       = (*app.APIProcess)(nil)
	_ mcpserver.Dependencies = (*app.MCPProcess)(nil)
)

func TestProcessComponentContractsAreDistinct(t *testing.T) {
	apiComponents := reflect.TypeOf((*app.APIComponents)(nil)).Elem()
	mcpComponents := reflect.TypeOf((*app.MCPComponents)(nil)).Elem()

	if apiComponents.Implements(mcpComponents) {
		t.Fatal("API composition contract unexpectedly includes the complete MCP surface")
	}
	if mcpComponents.Implements(apiComponents) {
		t.Fatal("MCP composition contract unexpectedly includes API lifecycle services")
	}
}

func TestMCPProcessDoesNotExposeAPILifecycleServices(t *testing.T) {
	mcpProcess := reflect.TypeOf((*app.MCPProcess)(nil))
	for _, method := range []string{"OutboxDrainer", "MarketData", "AlertEngine", "Copilot", "ProviderConnections"} {
		if _, ok := mcpProcess.MethodByName(method); ok {
			t.Fatalf("MCP process unexpectedly exposes API-only method %s", method)
		}
	}
}

func TestAPIProcessDoesNotExposeMCPOnlyServices(t *testing.T) {
	apiProcess := reflect.TypeOf((*app.APIProcess)(nil))
	for _, method := range []string{"PageDocuments", "Preview", "ShardCatalog", "QuoteSnapshots", "MarketInfo"} {
		if _, ok := apiProcess.MethodByName(method); ok {
			t.Fatalf("API process unexpectedly exposes MCP-only method %s", method)
		}
	}
}
