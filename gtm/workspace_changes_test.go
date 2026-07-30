package gtm

import (
	"testing"

	tagmanager "google.golang.org/api/tagmanager/v2"
)

// TestBuildWorkspaceChanges_WAE230 reproduces the acceptance criterion: a
// workspace with changeCount=20 made up of 6 added tags, 1 added trigger, and 13
// updated (paused) tags. A paused tag must surface as "updated" with paused=true,
// never as "deleted".
func TestBuildWorkspaceChanges_WAE230(t *testing.T) {
	var changes []*tagmanager.Entity
	for i := 0; i < 6; i++ {
		changes = append(changes, &tagmanager.Entity{
			ChangeStatus: "added",
			Tag:          &tagmanager.Tag{TagId: "a", Name: "added tag", Type: "html"},
		})
	}
	changes = append(changes, &tagmanager.Entity{
		ChangeStatus: "added",
		Trigger:      &tagmanager.Trigger{TriggerId: "t", Name: "added trigger", Type: "pageview"},
	})
	for i := 0; i < 13; i++ {
		changes = append(changes, &tagmanager.Entity{
			ChangeStatus: "updated",
			Tag:          &tagmanager.Tag{TagId: "p", Name: "paused lead tag", Type: "html", Paused: true},
		})
	}

	res := buildWorkspaceChanges(&tagmanager.GetWorkspaceStatusResponse{WorkspaceChange: changes}, false)

	if res.ChangeCount != 20 {
		t.Fatalf("ChangeCount = %d, want 20", res.ChangeCount)
	}
	if len(res.Changes) != 20 {
		t.Fatalf("len(Changes) = %d, want 20", len(res.Changes))
	}

	var addedTags, addedTriggers, updatedPausedTags int
	for _, c := range res.Changes {
		switch {
		case c.ChangeStatus == "added" && c.EntityType == "tag":
			addedTags++
		case c.ChangeStatus == "added" && c.EntityType == "trigger":
			addedTriggers++
		case c.ChangeStatus == "updated" && c.EntityType == "tag" && c.Paused:
			updatedPausedTags++
		default:
			t.Errorf("unexpected change: %+v", c)
		}
	}
	if addedTags != 6 || addedTriggers != 1 || updatedPausedTags != 13 {
		t.Errorf("breakdown = %d added tags, %d added triggers, %d updated paused tags; want 6/1/13",
			addedTags, addedTriggers, updatedPausedTags)
	}
}

// TestBuildWorkspaceChanges_SkipsUnchanged ensures "none"/unspecified entities
// are dropped so the count matches the real modifications.
func TestBuildWorkspaceChanges_SkipsUnchanged(t *testing.T) {
	res := buildWorkspaceChanges(&tagmanager.GetWorkspaceStatusResponse{
		WorkspaceChange: []*tagmanager.Entity{
			{ChangeStatus: "none", Tag: &tagmanager.Tag{TagId: "1"}},
			{ChangeStatus: "changeStatusUnspecified", Tag: &tagmanager.Tag{TagId: "2"}},
			nil,
			{ChangeStatus: "updated", Tag: &tagmanager.Tag{TagId: "3", Name: "real"}},
		},
	}, false)

	if res.ChangeCount != 1 || len(res.Changes) != 1 {
		t.Fatalf("ChangeCount = %d, want 1", res.ChangeCount)
	}
	if res.Changes[0].EntityID != "3" {
		t.Errorf("kept wrong entity: %+v", res.Changes[0])
	}
}

// TestSummarizeEntity_PerType checks the oneof mapping for the tricky types.
func TestSummarizeEntity_PerType(t *testing.T) {
	t.Run("compact omits body", func(t *testing.T) {
		c := summarizeEntity(&tagmanager.Entity{
			ChangeStatus: "updated",
			Variable:     &tagmanager.Variable{VariableId: "v9", Name: "My Var", Type: "c", Fingerprint: "fp"},
		}, false)
		if c.EntityType != "variable" || c.EntityID != "v9" || c.Name != "My Var" {
			t.Errorf("bad variable mapping: %+v", c)
		}
		if c.Body != nil {
			t.Errorf("compact mode must not include body, got %v", c.Body)
		}
	})

	t.Run("verbose keeps body with fingerprint", func(t *testing.T) {
		c := summarizeEntity(&tagmanager.Entity{
			ChangeStatus: "updated",
			Tag:          &tagmanager.Tag{TagId: "t1", Name: "n", Fingerprint: "fp123"},
		}, true)
		body, ok := c.Body.(*tagmanager.Tag)
		if !ok {
			t.Fatalf("verbose body should be *tagmanager.Tag, got %T", c.Body)
		}
		if body.Fingerprint != "fp123" {
			t.Errorf("fingerprint lost in verbose body: %+v", body)
		}
	})

	t.Run("gtagConfig falls back to type for name", func(t *testing.T) {
		c := summarizeEntity(&tagmanager.Entity{
			ChangeStatus: "added",
			GtagConfig:   &tagmanager.GtagConfig{GtagConfigId: "g1", Type: "googtag"},
		}, false)
		if c.EntityType != "gtagConfig" || c.EntityID != "g1" || c.Name != "googtag" {
			t.Errorf("bad gtagConfig mapping: %+v", c)
		}
	})

	t.Run("builtInVariable has no id", func(t *testing.T) {
		c := summarizeEntity(&tagmanager.Entity{
			ChangeStatus: "added",
			BuiltInVariable: &tagmanager.BuiltInVariable{
				Name: "Page URL", Type: "pageUrl",
			},
		}, false)
		if c.EntityType != "builtInVariable" || c.EntityID != "" || c.Name != "Page URL" {
			t.Errorf("bad builtInVariable mapping: %+v", c)
		}
	})
}

// TestBuildWorkspaceChanges_Conflicts verifies merge conflicts are summarised on
// both sides, and that a deleted-from-base conflict has no base entity.
func TestBuildWorkspaceChanges_Conflicts(t *testing.T) {
	res := buildWorkspaceChanges(&tagmanager.GetWorkspaceStatusResponse{
		MergeConflict: []*tagmanager.MergeConflict{
			{
				EntityInWorkspace:   &tagmanager.Entity{ChangeStatus: "updated", Tag: &tagmanager.Tag{TagId: "1", Name: "ws"}},
				EntityInBaseVersion: &tagmanager.Entity{ChangeStatus: "updated", Tag: &tagmanager.Tag{TagId: "1", Name: "base"}},
			},
			{
				EntityInWorkspace: &tagmanager.Entity{ChangeStatus: "deleted", Tag: &tagmanager.Tag{TagId: "2", Name: "gone"}},
			},
		},
	}, false)

	if res.ConflictCount != 2 || len(res.Conflicts) != 2 {
		t.Fatalf("ConflictCount = %d, want 2", res.ConflictCount)
	}
	if res.Conflicts[0].EntityInWorkspace == nil || res.Conflicts[0].EntityInBaseVersion == nil {
		t.Errorf("first conflict should have both sides: %+v", res.Conflicts[0])
	}
	if res.Conflicts[1].EntityInBaseVersion != nil {
		t.Errorf("deleted-from-base conflict should have no base entity: %+v", res.Conflicts[1])
	}
}
