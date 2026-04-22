// Package rag provides channel-scoped retrieval-augmented generation
// using DuckDB vector similarity search.
package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/nlink-jp/slack-personal-agent/internal/embedding"
)

// Retriever performs channel-scoped vector similarity search.
type Retriever struct {
	db       *sql.DB
	embedder embedding.Embedder
}

// NewRetriever creates a new retriever backed by the given DuckDB connection and embedder.
func NewRetriever(db *sql.DB, embedder embedding.Embedder) *Retriever {
	return &Retriever{db: db, embedder: embedder}
}

// Migrate creates the embeddings table and metadata table.
func (r *Retriever) Migrate() error {
	ddl := `
		CREATE TABLE IF NOT EXISTS embeddings (
			id            VARCHAR NOT NULL,
			record_id     VARCHAR NOT NULL,
			workspace_id  VARCHAR NOT NULL,
			channel_id    VARCHAR NOT NULL,
			embedding     FLOAT[] NOT NULL,
			model_id      VARCHAR NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_emb_ws_ch
			ON embeddings (workspace_id, channel_id);

		CREATE TABLE IF NOT EXISTS embedding_meta (
			key   VARCHAR NOT NULL,
			value VARCHAR NOT NULL
		);
	`
	_, err := r.db.Exec(ddl)
	return err
}

// SearchScope defines the knowledge isolation scope for a search.
type SearchScope struct {
	// Level 1: single channel (always required)
	WorkspaceID string
	ChannelID   string

	// Level 2: additional channels within the same workspace (optional)
	CrossChannelIDs []string

	// Level 3: additional workspaces (optional, each with their channels)
	CrossWorkspaces []WorkspaceScope
}

// WorkspaceScope defines channels accessible in another workspace.
type WorkspaceScope struct {
	WorkspaceID string
	ChannelIDs  []string
}

// SearchResult holds a single search result with its similarity score.
type SearchResult struct {
	RecordID    string
	WorkspaceID string
	ChannelID   string
	Score       float64
}

// Index stores an embedding for a record.
func (r *Retriever) Index(ctx context.Context, id, recordID, workspaceID, channelID string, text string) error {
	vecs, err := r.embedder.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return fmt.Errorf("empty embedding returned")
	}

	vecLiteral := floatsToListLiteral(vecs[0])
	_, err = r.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO embeddings (id, record_id, workspace_id, channel_id, embedding, model_id)
		VALUES (?, ?, ?, ?, %s::FLOAT[], ?)`, vecLiteral),
		id, recordID, workspaceID, channelID, r.embedder.ModelID())
	return err
}

// Search performs a channel-scoped vector similarity search.
// The scope parameter enforces the 3-tier knowledge isolation model.
func (r *Retriever) Search(ctx context.Context, query string, scope SearchScope, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	vecs, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}

	// Build scope filter
	conditions, args := buildScopeFilter(scope)
	args = append(args, limit)

	vecLiteral := floatsToListLiteral(vecs[0])
	query_sql := fmt.Sprintf(`
		SELECT record_id, workspace_id, channel_id,
			   list_cosine_similarity(embedding, %s::FLOAT[]) AS score
		FROM embeddings
		WHERE embedding IS NOT NULL AND len(embedding) > 0
		  AND (%s)
		ORDER BY score DESC
		LIMIT ?`, vecLiteral, conditions)

	allArgs := args

	rows, err := r.db.QueryContext(ctx, query_sql, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		if err := rows.Scan(&sr.RecordID, &sr.WorkspaceID, &sr.ChannelID, &sr.Score); err != nil {
			return nil, err
		}
		results = append(results, sr)
	}
	return results, rows.Err()
}

// DeleteByRecord removes embeddings for a given record ID.
func (r *Retriever) DeleteByRecord(ctx context.Context, recordID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM embeddings WHERE record_id = ?`, recordID)
	return err
}

// CheckModelConsistency verifies that stored embeddings match the current model.
// Returns the stored model ID and whether it matches.
func (r *Retriever) CheckModelConsistency(ctx context.Context) (storedModelID string, consistent bool, err error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT value FROM embedding_meta WHERE key = 'model_id' LIMIT 1`)

	err = row.Scan(&storedModelID)
	if err == sql.ErrNoRows {
		// No model recorded yet — first run, store current model
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO embedding_meta (key, value) VALUES ('model_id', ?)`,
			r.embedder.ModelID())
		return r.embedder.ModelID(), true, err
	}
	if err != nil {
		return "", false, err
	}

	return storedModelID, storedModelID == r.embedder.ModelID(), nil
}

// buildScopeFilter creates the WHERE clause for the 3-tier knowledge isolation.
func buildScopeFilter(scope SearchScope) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// Level 1: Primary channel (always included)
	conditions = append(conditions, "(workspace_id = ? AND channel_id = ?)")
	args = append(args, scope.WorkspaceID, scope.ChannelID)

	// Level 2: Cross-channel within same workspace
	for _, chID := range scope.CrossChannelIDs {
		conditions = append(conditions, "(workspace_id = ? AND channel_id = ?)")
		args = append(args, scope.WorkspaceID, chID)
	}

	// Level 3: Cross-workspace
	for _, ws := range scope.CrossWorkspaces {
		for _, chID := range ws.ChannelIDs {
			conditions = append(conditions, "(workspace_id = ? AND channel_id = ?)")
			args = append(args, ws.WorkspaceID, chID)
		}
	}

	return strings.Join(conditions, " OR "), args
}

// floatsToListLiteral converts a float32 slice to a DuckDB list literal string.
// e.g., [0.1, 0.2, 0.3] → "[0.1,0.2,0.3]"
func floatsToListLiteral(vec []float32) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
