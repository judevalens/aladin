package mcpserver

import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

func destructiveTool(title string) *sdkmcp.ToolAnnotations {
	destructive := true
	openWorld := false
	return &sdkmcp.ToolAnnotations{
		Title:           title,
		DestructiveHint: &destructive,
		OpenWorldHint:   &openWorld,
	}
}
