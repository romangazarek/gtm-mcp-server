package gtm

import (
	"context"

	tagmanager "google.golang.org/api/tagmanager/v2"
)

// WorkspaceChange is a compact description of a single changed entity in a
// workspace. By default it carries just enough to review the change; when the
// caller requests verbose output, Body holds the full entity (with fingerprint).
type WorkspaceChange struct {
	ChangeStatus string `json:"changeStatus"`       // added | deleted | updated
	EntityType   string `json:"entityType"`         // tag | trigger | variable | folder | client | template | transformation | zone | gtagConfig | builtInVariable
	EntityID     string `json:"entityId,omitempty"` // empty for builtInVariable (it has no ID)
	Name         string `json:"name,omitempty"`     // gtagConfig has no name; falls back to its type
	Type         string `json:"type,omitempty"`     // the entity's own GTM type, where it has one
	Paused       bool   `json:"paused,omitempty"`   // tags only; a paused tag is an "updated" change, not "deleted"
	Body         any    `json:"body,omitempty"`     // verbose only: the raw entity body, including fingerprint
}

// WorkspaceConflict mirrors a GTM merge conflict, summarising each side.
type WorkspaceConflict struct {
	EntityInWorkspace   *WorkspaceChange `json:"entityInWorkspace,omitempty"`
	EntityInBaseVersion *WorkspaceChange `json:"entityInBaseVersion,omitempty"` // missing when the entity was deleted from the base version
}

// WorkspaceChangesResult is the full changeset of a workspace versus its base version.
type WorkspaceChangesResult struct {
	ChangeCount   int                 `json:"changeCount"`
	Changes       []WorkspaceChange   `json:"changes"`
	ConflictCount int                 `json:"conflictCount"`
	Conflicts     []WorkspaceConflict `json:"conflicts,omitempty"`
}

// GetWorkspaceChanges returns the exact list of entities that changed in a
// workspace (added/updated/deleted) plus any merge conflicts. Unlike
// GetWorkspaceStatus, which collapses the same API response into counts, this
// passes through the per-entity detail the GTM getStatus endpoint already
// returns in a single call.
func (c *Client) GetWorkspaceChanges(ctx context.Context, accountID, containerID, workspaceID string, verbose bool) (*WorkspaceChangesResult, error) {
	path := BuildWorkspacePath(accountID, containerID, workspaceID)

	status, err := retryWithBackoff(ctx, 3, func() (*tagmanager.GetWorkspaceStatusResponse, error) {
		return c.Service.Accounts.Containers.Workspaces.GetStatus(path).Context(ctx).Do()
	})
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return buildWorkspaceChanges(status, verbose), nil
}

// buildWorkspaceChanges turns a raw GTM getStatus response into the compact
// changeset. Split from GetWorkspaceChanges so it can be unit-tested without a
// live API client.
func buildWorkspaceChanges(status *tagmanager.GetWorkspaceStatusResponse, verbose bool) *WorkspaceChangesResult {
	result := &WorkspaceChangesResult{
		Changes:   make([]WorkspaceChange, 0, len(status.WorkspaceChange)),
		Conflicts: make([]WorkspaceConflict, 0, len(status.MergeConflict)),
	}

	for _, e := range status.WorkspaceChange {
		// Skip unchanged entities so the change list (and its count) reflects
		// only real modifications.
		if e == nil || e.ChangeStatus == "none" || e.ChangeStatus == "changeStatusUnspecified" {
			continue
		}
		result.Changes = append(result.Changes, summarizeEntity(e, verbose))
	}
	result.ChangeCount = len(result.Changes)

	for _, mc := range status.MergeConflict {
		if mc == nil {
			continue
		}
		conflict := WorkspaceConflict{}
		if mc.EntityInWorkspace != nil {
			s := summarizeEntity(mc.EntityInWorkspace, verbose)
			conflict.EntityInWorkspace = &s
		}
		if mc.EntityInBaseVersion != nil {
			s := summarizeEntity(mc.EntityInBaseVersion, verbose)
			conflict.EntityInBaseVersion = &s
		}
		result.Conflicts = append(result.Conflicts, conflict)
	}
	result.ConflictCount = len(result.Conflicts)

	return result
}

// summarizeEntity maps a GTM Entity (a oneof over the various entity types) into
// a compact WorkspaceChange. When verbose is true the raw entity body is kept so
// callers can do a field-level diff (it includes the fingerprint).
func summarizeEntity(e *tagmanager.Entity, verbose bool) WorkspaceChange {
	wc := WorkspaceChange{ChangeStatus: e.ChangeStatus}

	switch {
	case e.Tag != nil:
		wc.EntityType = "tag"
		wc.EntityID = e.Tag.TagId
		wc.Name = e.Tag.Name
		wc.Type = e.Tag.Type
		wc.Paused = e.Tag.Paused
		if verbose {
			wc.Body = e.Tag
		}
	case e.Trigger != nil:
		wc.EntityType = "trigger"
		wc.EntityID = e.Trigger.TriggerId
		wc.Name = e.Trigger.Name
		wc.Type = e.Trigger.Type
		if verbose {
			wc.Body = e.Trigger
		}
	case e.Variable != nil:
		wc.EntityType = "variable"
		wc.EntityID = e.Variable.VariableId
		wc.Name = e.Variable.Name
		wc.Type = e.Variable.Type
		if verbose {
			wc.Body = e.Variable
		}
	case e.Folder != nil:
		wc.EntityType = "folder"
		wc.EntityID = e.Folder.FolderId
		wc.Name = e.Folder.Name
		if verbose {
			wc.Body = e.Folder
		}
	case e.Client != nil:
		wc.EntityType = "client"
		wc.EntityID = e.Client.ClientId
		wc.Name = e.Client.Name
		wc.Type = e.Client.Type
		if verbose {
			wc.Body = e.Client
		}
	case e.CustomTemplate != nil:
		wc.EntityType = "template"
		wc.EntityID = e.CustomTemplate.TemplateId
		wc.Name = e.CustomTemplate.Name
		if verbose {
			wc.Body = e.CustomTemplate
		}
	case e.Transformation != nil:
		wc.EntityType = "transformation"
		wc.EntityID = e.Transformation.TransformationId
		wc.Name = e.Transformation.Name
		wc.Type = e.Transformation.Type
		if verbose {
			wc.Body = e.Transformation
		}
	case e.Zone != nil:
		wc.EntityType = "zone"
		wc.EntityID = e.Zone.ZoneId
		wc.Name = e.Zone.Name
		if verbose {
			wc.Body = e.Zone
		}
	case e.GtagConfig != nil:
		wc.EntityType = "gtagConfig"
		wc.EntityID = e.GtagConfig.GtagConfigId
		wc.Type = e.GtagConfig.Type
		wc.Name = e.GtagConfig.Type // GtagConfig has no name field
		if verbose {
			wc.Body = e.GtagConfig
		}
	case e.BuiltInVariable != nil:
		wc.EntityType = "builtInVariable"
		wc.Name = e.BuiltInVariable.Name
		wc.Type = e.BuiltInVariable.Type
		// built-in variables have neither an ID nor a fingerprint
		if verbose {
			wc.Body = e.BuiltInVariable
		}
	default:
		wc.EntityType = "unknown"
	}

	return wc
}
