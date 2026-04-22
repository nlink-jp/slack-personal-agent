package rag

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/nlink-jp/slack-personal-agent/internal/embedding"

	_ "github.com/marcboeker/go-duckdb"
)

func testRetriever(t *testing.T) (*Retriever, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	emb := embedding.NewMockEmbedder(384)
	r := NewRetriever(db, emb)
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}
	return r, db
}

func TestIndexAndSearch(t *testing.T) {
	r, _ := testRetriever(t)
	ctx := context.Background()

	// Index some records in different channels
	r.Index(ctx, uuid.New().String(), "rec-1", "WS1", "CH1", "Slack message about security incident")
	r.Index(ctx, uuid.New().String(), "rec-2", "WS1", "CH1", "Follow-up on the security review")
	r.Index(ctx, uuid.New().String(), "rec-3", "WS1", "CH2", "Budget planning for next quarter")

	// Search with Level 1 scope (CH1 only)
	scope := SearchScope{
		WorkspaceID: "WS1",
		ChannelID:   "CH1",
	}
	results, err := r.Search(ctx, "security issue", scope, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Should only find records from CH1
	if len(results) != 2 {
		t.Fatalf("expected 2 results from CH1, got %d", len(results))
	}
	for _, res := range results {
		if res.ChannelID != "CH1" {
			t.Errorf("expected CH1, got %q (channel isolation violated)", res.ChannelID)
		}
	}
}

func TestSearchChannelIsolation(t *testing.T) {
	r, _ := testRetriever(t)
	ctx := context.Background()

	// Index in different workspaces and channels
	r.Index(ctx, uuid.New().String(), "rec-ws1-ch1", "WS1", "CH1", "confidential data")
	r.Index(ctx, uuid.New().String(), "rec-ws1-ch2", "WS1", "CH2", "public info")
	r.Index(ctx, uuid.New().String(), "rec-ws2-ch1", "WS2", "CH1", "other workspace data")

	// Level 1: only CH1 in WS1
	scope := SearchScope{
		WorkspaceID: "WS1",
		ChannelID:   "CH1",
	}
	results, err := r.Search(ctx, "data", scope, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (Level 1), got %d", len(results))
	}
	if results[0].RecordID != "rec-ws1-ch1" {
		t.Errorf("expected rec-ws1-ch1, got %q", results[0].RecordID)
	}
}

func TestSearchCrossChannel(t *testing.T) {
	r, _ := testRetriever(t)
	ctx := context.Background()

	r.Index(ctx, uuid.New().String(), "rec-ch1", "WS1", "CH1", "message in channel 1")
	r.Index(ctx, uuid.New().String(), "rec-ch2", "WS1", "CH2", "message in channel 2")
	r.Index(ctx, uuid.New().String(), "rec-ch3", "WS1", "CH3", "message in channel 3")

	// Level 2: CH1 + CH2 (not CH3)
	scope := SearchScope{
		WorkspaceID:     "WS1",
		ChannelID:       "CH1",
		CrossChannelIDs: []string{"CH2"},
	}
	results, err := r.Search(ctx, "message", scope, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (Level 2), got %d", len(results))
	}

	// CH3 should not appear
	for _, res := range results {
		if res.ChannelID == "CH3" {
			t.Error("CH3 should not be in results (not in scope)")
		}
	}
}

func TestSearchCrossWorkspace(t *testing.T) {
	r, _ := testRetriever(t)
	ctx := context.Background()

	r.Index(ctx, uuid.New().String(), "rec-ws1", "WS1", "CH1", "workspace 1 message")
	r.Index(ctx, uuid.New().String(), "rec-ws2", "WS2", "CHX", "workspace 2 message")
	r.Index(ctx, uuid.New().String(), "rec-ws3", "WS3", "CHY", "workspace 3 message")

	// Level 3: WS1/CH1 + WS2/CHX (not WS3)
	scope := SearchScope{
		WorkspaceID: "WS1",
		ChannelID:   "CH1",
		CrossWorkspaces: []WorkspaceScope{
			{WorkspaceID: "WS2", ChannelIDs: []string{"CHX"}},
		},
	}
	results, err := r.Search(ctx, "message", scope, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (Level 3), got %d", len(results))
	}

	// WS3 should not appear
	for _, res := range results {
		if res.WorkspaceID == "WS3" {
			t.Error("WS3 should not be in results (not in scope)")
		}
	}
}

func TestDeleteByRecord(t *testing.T) {
	r, _ := testRetriever(t)
	ctx := context.Background()

	r.Index(ctx, uuid.New().String(), "rec-1", "WS1", "CH1", "message one")
	r.Index(ctx, uuid.New().String(), "rec-2", "WS1", "CH1", "message two")

	if err := r.DeleteByRecord(ctx, "rec-1"); err != nil {
		t.Fatal(err)
	}

	scope := SearchScope{WorkspaceID: "WS1", ChannelID: "CH1"}
	results, err := r.Search(ctx, "message", scope, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result after delete, got %d", len(results))
	}
	if results[0].RecordID != "rec-2" {
		t.Errorf("expected rec-2 to remain, got %q", results[0].RecordID)
	}
}

func TestCheckModelConsistency(t *testing.T) {
	r, _ := testRetriever(t)
	ctx := context.Background()

	// First call — no model stored yet, should store and return consistent
	storedID, consistent, err := r.CheckModelConsistency(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !consistent {
		t.Error("expected consistent on first run")
	}
	if storedID != "mock:test:384" {
		t.Errorf("expected mock model ID, got %q", storedID)
	}

	// Second call — same model, should be consistent
	storedID, consistent, err = r.CheckModelConsistency(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !consistent {
		t.Error("expected consistent on second call")
	}
}

func TestBuildScopeFilter(t *testing.T) {
	// Level 1 only
	cond, args := buildScopeFilter(SearchScope{
		WorkspaceID: "WS1",
		ChannelID:   "CH1",
	})
	if cond != "(workspace_id = ? AND channel_id = ?)" {
		t.Errorf("unexpected Level 1 condition: %q", cond)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}

	// Level 2
	cond, args = buildScopeFilter(SearchScope{
		WorkspaceID:     "WS1",
		ChannelID:       "CH1",
		CrossChannelIDs: []string{"CH2", "CH3"},
	})
	if len(args) != 6 { // 2 + 2 + 2
		t.Errorf("expected 6 args for Level 2, got %d", len(args))
	}

	// Level 3
	cond, args = buildScopeFilter(SearchScope{
		WorkspaceID: "WS1",
		ChannelID:   "CH1",
		CrossWorkspaces: []WorkspaceScope{
			{WorkspaceID: "WS2", ChannelIDs: []string{"CHX"}},
		},
	})
	_ = cond
	if len(args) != 4 { // 2 + 2
		t.Errorf("expected 4 args for Level 3, got %d", len(args))
	}
}
