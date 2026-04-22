package knowledge

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	s := NewStore(db)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddAndGet(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	e, err := s.Add(ctx, "Security Policy", "All endpoints must use TLS", ScopeGlobal, "", []string{"security", "policy"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "Security Policy" {
		t.Errorf("expected title 'Security Policy', got %q", got.Title)
	}
	if got.Scope != ScopeGlobal {
		t.Errorf("expected global scope, got %q", got.Scope)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "security" {
		t.Errorf("expected tags [security, policy], got %v", got.Tags)
	}
}

func TestWorkspaceScopeRequiresID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Add(ctx, "test", "content", ScopeWorkspace, "", nil)
	if err == nil {
		t.Error("expected error for workspace scope without workspace_id")
	}
}

func TestUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	e, _ := s.Add(ctx, "Original", "original content", ScopeGlobal, "", nil)

	err := s.Update(ctx, e.ID, "Updated", "new content", ScopeWorkspace, "WS1", []string{"updated"})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(ctx, e.ID)
	if got.Title != "Updated" {
		t.Errorf("expected 'Updated', got %q", got.Title)
	}
	if got.Scope != ScopeWorkspace {
		t.Errorf("expected workspace scope, got %q", got.Scope)
	}
	if got.WorkspaceID != "WS1" {
		t.Errorf("expected WS1, got %q", got.WorkspaceID)
	}
}

func TestDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	e, _ := s.Add(ctx, "to delete", "content", ScopeGlobal, "", nil)
	if err := s.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}

	_, err := s.Get(ctx, e.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Add(ctx, "Global 1", "g1", ScopeGlobal, "", nil)
	s.Add(ctx, "Global 2", "g2", ScopeGlobal, "", nil)
	s.Add(ctx, "WS entry", "ws", ScopeWorkspace, "WS1", nil)

	// List all
	all, err := s.List(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}

	// List global only
	globalScope := ScopeGlobal
	globals, err := s.List(ctx, &globalScope)
	if err != nil {
		t.Fatal(err)
	}
	if len(globals) != 2 {
		t.Errorf("expected 2 global entries, got %d", len(globals))
	}
}

func TestFindForScope(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Add(ctx, "WS1 knowledge", "ws1", ScopeWorkspace, "WS1", nil)
	s.Add(ctx, "WS2 knowledge", "ws2", ScopeWorkspace, "WS2", nil)
	s.Add(ctx, "Global knowledge", "global", ScopeGlobal, "", nil)

	// WS1 without global
	entries, err := s.FindForScope(ctx, "WS1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for WS1 (no global), got %d", len(entries))
	}
	if entries[0].Title != "WS1 knowledge" {
		t.Errorf("expected WS1 knowledge, got %q", entries[0].Title)
	}

	// WS1 with global
	entries, err = s.FindForScope(ctx, "WS1", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for WS1 (with global), got %d", len(entries))
	}

	// WS2 should not see WS1 entries
	entries, err = s.FindForScope(ctx, "WS2", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for WS2, got %d", len(entries))
	}
	if entries[0].Title != "WS2 knowledge" {
		t.Errorf("expected WS2 knowledge, got %q", entries[0].Title)
	}
}
