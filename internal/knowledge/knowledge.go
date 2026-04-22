// Package knowledge manages the internal knowledge base (non-Slack knowledge).
// Knowledge entries are stored in DuckDB alongside Slack messages and can
// participate in RAG search at configurable scope levels (L2/L3).
package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Scope defines the visibility of a knowledge entry in the isolation model.
type Scope string

const (
	// ScopeWorkspace makes the entry available within a specific workspace (Level 2).
	ScopeWorkspace Scope = "workspace"
	// ScopeGlobal makes the entry available across all workspaces (Level 3).
	ScopeGlobal Scope = "global"
)

// Entry represents a knowledge base entry.
type Entry struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Scope       Scope     `json:"scope"`
	WorkspaceID string    `json:"workspace_id,omitempty"` // Only for ScopeWorkspace
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store manages knowledge entries in DuckDB.
type Store struct {
	db *sql.DB
}

// NewStore creates a new knowledge store using the given DuckDB connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate creates the knowledge table.
func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS knowledge (
			id           VARCHAR NOT NULL,
			title        VARCHAR NOT NULL,
			content      VARCHAR NOT NULL,
			scope        VARCHAR NOT NULL DEFAULT 'global',
			workspace_id VARCHAR NOT NULL DEFAULT '',
			tags         VARCHAR NOT NULL DEFAULT '[]',
			created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_knowledge_scope
			ON knowledge (scope, workspace_id);
	`)
	return err
}

// Add creates a new knowledge entry.
func (s *Store) Add(ctx context.Context, title, content string, scope Scope, workspaceID string, tags []string) (*Entry, error) {
	if scope == ScopeWorkspace && workspaceID == "" {
		return nil, fmt.Errorf("workspace_id required for workspace scope")
	}

	e := &Entry{
		ID:          uuid.New().String(),
		Title:       title,
		Content:     content,
		Scope:       scope,
		WorkspaceID: workspaceID,
		Tags:        tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tagsJSON := "[]"
	if len(tags) > 0 {
		tagsJSON = `["` + strings.Join(tags, `","`) + `"]`
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge (id, title, content, scope, workspace_id, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.Content, string(e.Scope), e.WorkspaceID, tagsJSON, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Update modifies an existing knowledge entry.
func (s *Store) Update(ctx context.Context, id, title, content string, scope Scope, workspaceID string, tags []string) error {
	tagsJSON := "[]"
	if len(tags) > 0 {
		tagsJSON = `["` + strings.Join(tags, `","`) + `"]`
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE knowledge SET title = ?, content = ?, scope = ?, workspace_id = ?,
			tags = ?, updated_at = ?
		WHERE id = ?`,
		title, content, string(scope), workspaceID, tagsJSON, time.Now(), id)
	return err
}

// Delete removes a knowledge entry.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM knowledge WHERE id = ?`, id)
	return err
}

// Get retrieves a single knowledge entry by ID.
func (s *Store) Get(ctx context.Context, id string) (*Entry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, content, scope, workspace_id, tags, created_at, updated_at
		FROM knowledge WHERE id = ?`, id)

	return scanEntry(row)
}

// List returns all knowledge entries, optionally filtered by scope.
func (s *Store) List(ctx context.Context, scope *Scope) ([]Entry, error) {
	var query string
	var args []interface{}

	if scope != nil {
		query = `SELECT id, title, content, scope, workspace_id, tags, created_at, updated_at
			FROM knowledge WHERE scope = ? ORDER BY updated_at DESC`
		args = append(args, string(*scope))
	} else {
		query = `SELECT id, title, content, scope, workspace_id, tags, created_at, updated_at
			FROM knowledge ORDER BY updated_at DESC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var scope string
		var tagsJSON string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &scope, &e.WorkspaceID, &tagsJSON, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Scope = Scope(scope)
		e.Tags = parseTags(tagsJSON)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// FindForScope returns knowledge entries visible to the given scope.
func (s *Store) FindForScope(ctx context.Context, workspaceID string, includeGlobal bool) ([]Entry, error) {
	var conditions []string
	var args []interface{}

	// Workspace-scoped entries for the given workspace
	conditions = append(conditions, "(scope = 'workspace' AND workspace_id = ?)")
	args = append(args, workspaceID)

	// Global entries if permitted
	if includeGlobal {
		conditions = append(conditions, "scope = 'global'")
	}

	query := fmt.Sprintf(`
		SELECT id, title, content, scope, workspace_id, tags, created_at, updated_at
		FROM knowledge WHERE %s ORDER BY updated_at DESC`,
		strings.Join(conditions, " OR "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var scope string
		var tagsJSON string
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &scope, &e.WorkspaceID, &tagsJSON, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Scope = Scope(scope)
		e.Tags = parseTags(tagsJSON)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func scanEntry(row *sql.Row) (*Entry, error) {
	var e Entry
	var scope string
	var tagsJSON string
	if err := row.Scan(&e.ID, &e.Title, &e.Content, &scope, &e.WorkspaceID, &tagsJSON, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	e.Scope = Scope(scope)
	e.Tags = parseTags(tagsJSON)
	return &e, nil
}

func parseTags(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	s = strings.TrimPrefix(s, `["`)
	s = strings.TrimSuffix(s, `"]`)
	if s == "" {
		return nil
	}
	return strings.Split(s, `","`)
}
