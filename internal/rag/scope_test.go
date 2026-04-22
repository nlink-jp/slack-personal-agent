package rag

import (
	"testing"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

func TestBuildScopeNoGroups(t *testing.T) {
	scope := BuildScope("WS1", "CH1", nil)

	if scope.WorkspaceID != "WS1" {
		t.Errorf("expected WS1, got %q", scope.WorkspaceID)
	}
	if scope.ChannelID != "CH1" {
		t.Errorf("expected CH1, got %q", scope.ChannelID)
	}
	if len(scope.CrossChannelIDs) != 0 {
		t.Errorf("expected no cross channels, got %v", scope.CrossChannelIDs)
	}
	if len(scope.CrossWorkspaces) != 0 {
		t.Errorf("expected no cross workspaces, got %v", scope.CrossWorkspaces)
	}
}

func TestBuildScopeLevel2(t *testing.T) {
	groups := []config.ScopeGroup{
		{
			Name: "security-team",
			Members: []config.ScopeMember{
				{WorkspaceID: "WS1", ChannelID: "CH1"},
				{WorkspaceID: "WS1", ChannelID: "CH2"},
				{WorkspaceID: "WS1", ChannelID: "CH3"},
			},
		},
	}

	scope := BuildScope("WS1", "CH1", groups)

	if len(scope.CrossChannelIDs) != 2 {
		t.Fatalf("expected 2 cross channels, got %d", len(scope.CrossChannelIDs))
	}
	if scope.CrossChannelIDs[0] != "CH2" || scope.CrossChannelIDs[1] != "CH3" {
		t.Errorf("expected [CH2, CH3], got %v", scope.CrossChannelIDs)
	}
}

func TestBuildScopeLevel3(t *testing.T) {
	groups := []config.ScopeGroup{
		{
			Name: "cross-ws-infra",
			Members: []config.ScopeMember{
				{WorkspaceID: "WS1", ChannelID: "CH1"},
				{WorkspaceID: "WS2", ChannelID: "CHX"},
				{WorkspaceID: "WS3", ChannelID: "CHY"},
			},
		},
	}

	scope := BuildScope("WS1", "CH1", groups)

	if len(scope.CrossChannelIDs) != 0 {
		t.Errorf("expected no L2 channels, got %v", scope.CrossChannelIDs)
	}
	if len(scope.CrossWorkspaces) != 2 {
		t.Fatalf("expected 2 cross workspaces, got %d", len(scope.CrossWorkspaces))
	}
	if scope.CrossWorkspaces[0].WorkspaceID != "WS2" {
		t.Errorf("expected WS2, got %q", scope.CrossWorkspaces[0].WorkspaceID)
	}
}

func TestBuildScopeMixed(t *testing.T) {
	groups := []config.ScopeGroup{
		{
			Name: "mixed-group",
			Members: []config.ScopeMember{
				{WorkspaceID: "WS1", ChannelID: "CH1"},
				{WorkspaceID: "WS1", ChannelID: "CH2"}, // L2
				{WorkspaceID: "WS2", ChannelID: "CHX"}, // L3
			},
		},
	}

	scope := BuildScope("WS1", "CH1", groups)

	if len(scope.CrossChannelIDs) != 1 || scope.CrossChannelIDs[0] != "CH2" {
		t.Errorf("expected L2 [CH2], got %v", scope.CrossChannelIDs)
	}
	if len(scope.CrossWorkspaces) != 1 || scope.CrossWorkspaces[0].WorkspaceID != "WS2" {
		t.Errorf("expected L3 [WS2], got %v", scope.CrossWorkspaces)
	}
}

func TestBuildScopeChannelNotInGroup(t *testing.T) {
	groups := []config.ScopeGroup{
		{
			Name: "other-group",
			Members: []config.ScopeMember{
				{WorkspaceID: "WS1", ChannelID: "CH9"},
				{WorkspaceID: "WS1", ChannelID: "CH10"},
			},
		},
	}

	// CH1 is not in this group, so no cross-channel access
	scope := BuildScope("WS1", "CH1", groups)

	if len(scope.CrossChannelIDs) != 0 {
		t.Errorf("expected no cross channels, got %v", scope.CrossChannelIDs)
	}
}

func TestBuildScopeMultipleGroups(t *testing.T) {
	groups := []config.ScopeGroup{
		{
			Name: "group-a",
			Members: []config.ScopeMember{
				{WorkspaceID: "WS1", ChannelID: "CH1"},
				{WorkspaceID: "WS1", ChannelID: "CH2"},
			},
		},
		{
			Name: "group-b",
			Members: []config.ScopeMember{
				{WorkspaceID: "WS1", ChannelID: "CH1"},
				{WorkspaceID: "WS1", ChannelID: "CH3"},
			},
		},
	}

	scope := BuildScope("WS1", "CH1", groups)

	// Should accumulate from both groups
	if len(scope.CrossChannelIDs) != 2 {
		t.Fatalf("expected 2 cross channels from 2 groups, got %d: %v", len(scope.CrossChannelIDs), scope.CrossChannelIDs)
	}
}
