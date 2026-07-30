package gtm

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetWorkspaceChangesInput struct {
	AccountID   string `json:"accountId" jsonschema:"description:The GTM account ID"`
	ContainerID string `json:"containerId" jsonschema:"description:The GTM container ID"`
	WorkspaceID string `json:"workspaceId" jsonschema:"description:The GTM workspace ID"`
	Verbose     bool   `json:"verbose,omitempty" jsonschema:"description:When true, include the full entity body (with fingerprint) for each change; default false returns a compact summary"`
}

type GetWorkspaceChangesOutput struct {
	ChangeCount   int                 `json:"changeCount"`
	Changes       []WorkspaceChange   `json:"changes"`
	ConflictCount int                 `json:"conflictCount"`
	Conflicts     []WorkspaceConflict `json:"conflicts,omitempty"`
}

func registerGetWorkspaceChanges(server *mcp.Server) {
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input GetWorkspaceChangesInput) (*mcp.CallToolResult, GetWorkspaceChangesOutput, error) {
		wc, err := resolveWorkspace(ctx, input.AccountID, input.ContainerID, input.WorkspaceID)
		if err != nil {
			return nil, GetWorkspaceChangesOutput{}, err
		}

		changes, err := wc.Client.GetWorkspaceChanges(ctx, wc.AccountID, wc.ContainerID, wc.WorkspaceID, input.Verbose)
		if err != nil {
			return nil, GetWorkspaceChangesOutput{}, err
		}

		return nil, GetWorkspaceChangesOutput{
			ChangeCount:   changes.ChangeCount,
			Changes:       changes.Changes,
			ConflictCount: changes.ConflictCount,
			Conflicts:     changes.Conflicts,
		}, nil
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workspace_changes",
		Description: "List exactly which entities changed in a workspace versus the live (base) version - added, updated, or deleted - along with any merge conflicts. Returns a compact summary per change by default; set verbose=true to include each entity's full body (with fingerprint) for a field-level diff. Use get_workspace_status instead when you only need counts.",
	}, handler)
}
