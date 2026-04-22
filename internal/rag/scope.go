package rag

import "github.com/nlink-jp/slack-personal-agent/internal/config"

// BuildScope creates a SearchScope from config scope groups for a given
// workspace and channel. This resolves Level 2 (cross-channel) and
// Level 3 (cross-workspace) permissions from the user's configuration.
func BuildScope(workspaceID, channelID string, groups []config.ScopeGroup) SearchScope {
	scope := SearchScope{
		WorkspaceID: workspaceID,
		ChannelID:   channelID,
	}

	// Find all groups that include the source channel
	for _, g := range groups {
		if !groupContains(g, workspaceID, channelID) {
			continue
		}

		// Add all other members of this group to the scope
		for _, m := range g.Members {
			if m.WorkspaceID == workspaceID && m.ChannelID == channelID {
				continue // skip self
			}

			if m.WorkspaceID == workspaceID {
				// Level 2: same workspace, different channel
				scope.CrossChannelIDs = append(scope.CrossChannelIDs, m.ChannelID)
			} else {
				// Level 3: different workspace
				addCrossWorkspace(&scope, m.WorkspaceID, m.ChannelID)
			}
		}
	}

	return scope
}

// groupContains checks if a scope group includes the given workspace+channel.
func groupContains(g config.ScopeGroup, wsID, chID string) bool {
	for _, m := range g.Members {
		if m.WorkspaceID == wsID && m.ChannelID == chID {
			return true
		}
	}
	return false
}

// addCrossWorkspace appends a channel to the cross-workspace scope,
// grouping by workspace ID.
func addCrossWorkspace(scope *SearchScope, wsID, chID string) {
	for i, ws := range scope.CrossWorkspaces {
		if ws.WorkspaceID == wsID {
			scope.CrossWorkspaces[i].ChannelIDs = append(ws.ChannelIDs, chID)
			return
		}
	}
	scope.CrossWorkspaces = append(scope.CrossWorkspaces, WorkspaceScope{
		WorkspaceID: wsID,
		ChannelIDs:  []string{chID},
	})
}
