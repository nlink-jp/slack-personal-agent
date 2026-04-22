package keychain

import "testing"

func TestMockStore(t *testing.T) {
	store := NewMockStore()

	// Set and Get
	if err := store.Set("workspace:test-ws", "xoxp-test-token"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("workspace:test-ws")
	if err != nil {
		t.Fatal(err)
	}
	if got != "xoxp-test-token" {
		t.Errorf("expected 'xoxp-test-token', got %q", got)
	}

	// Get non-existent
	_, err = store.Get("workspace:missing")
	if err == nil {
		t.Error("expected error for missing key")
	}

	// Delete
	if err := store.Delete("workspace:test-ws"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get("workspace:test-ws")
	if err == nil {
		t.Error("expected error after delete")
	}

	// Delete non-existent
	if err := store.Delete("workspace:missing"); err == nil {
		t.Error("expected error for deleting missing key")
	}
}

func TestWorkspaceTokenKey(t *testing.T) {
	key := WorkspaceTokenKey("company-a")
	if key != "workspace:company-a" {
		t.Errorf("expected 'workspace:company-a', got %q", key)
	}
}

func TestLLMAPIKeyKey(t *testing.T) {
	key := LLMAPIKeyKey("vertex_ai")
	if key != "llm:vertex_ai" {
		t.Errorf("expected 'llm:vertex_ai', got %q", key)
	}
}

// Verify OSStore implements Store interface at compile time.
var _ Store = (*OSStore)(nil)
var _ Store = (*MockStore)(nil)
