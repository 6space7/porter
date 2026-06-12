package mcpserver

import (
	"encoding/json"
	"fmt"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

func jsonResult(value any) (*mcpsdk.CallToolResult, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}
	result := mcpsdk.NewToolResultText(string(body))
	result.StructuredContent = value
	return result, nil
}
