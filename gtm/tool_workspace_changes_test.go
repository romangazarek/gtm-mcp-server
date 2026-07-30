package gtm

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRegisterGetWorkspaceChanges ensures the tool registers and its input/output
// schemas reflect cleanly from the Go types (AddTool infers them at runtime).
func TestRegisterGetWorkspaceChanges(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerGetWorkspaceChanges(server) // panics if schema inference fails
}
