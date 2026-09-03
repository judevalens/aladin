package mcpserver

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var updateDocSurfaceContract = flag.Bool("update-doc-surface-contract", false, "update the Doc Surface MCP contract snapshot")

type docSurfaceToolContract struct {
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Annotations  *sdkmcp.ToolAnnotations `json:"annotations,omitempty"`
	InputSchema  any                     `json:"input_schema"`
	OutputSchema any                     `json:"output_schema"`
}

func TestDocSurfaceToolContract(t *testing.T) {
	ctx := context.Background()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "doc-surface-contract", Version: "test"}, nil)
	registerDocSurfaceTools(server, nil, nil, nil, nil, nil, nil)

	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "doc-surface-contract-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	contracts := make([]docSurfaceToolContract, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		contracts = append(contracts, docSurfaceToolContract{
			Name:         tool.Name,
			Description:  tool.Description,
			Annotations:  tool.Annotations,
			InputSchema:  tool.InputSchema,
			OutputSchema: tool.OutputSchema,
		})
	}
	got, err := json.MarshalIndent(contracts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "doc_surface_tools.golden.json")
	if *updateDocSurfaceContract {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("Doc Surface MCP contract changed; review compatibility and run go test ./internal/mcp -run TestDocSurfaceToolContract -update-doc-surface-contract to accept it")
	}
}
