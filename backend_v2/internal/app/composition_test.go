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
	_ api.Dependencies       = (app.APIComponents)(nil)
	_ mcpserver.Dependencies = (app.MCPComponents)(nil)
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
