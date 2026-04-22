package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// Store manages the DuckDB-based memory storage.
type Store struct {
	db *sql.DB
}

// Open opens or creates a DuckDB database at the given path.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sql.DB for shared use (e.g., RAG retriever).
func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	ddl := `
		CREATE TABLE IF NOT EXISTS records (
			id            VARCHAR NOT NULL,
			workspace_id  VARCHAR NOT NULL,
			workspace_name VARCHAR NOT NULL,
			channel_id    VARCHAR NOT NULL,
			channel_name  VARCHAR NOT NULL,
			user_id       VARCHAR NOT NULL DEFAULT '',
			user_name     VARCHAR NOT NULL DEFAULT '',
			ts            VARCHAR NOT NULL DEFAULT '',
			thread_ts     VARCHAR NOT NULL DEFAULT '',
			content       VARCHAR NOT NULL,
			tier          VARCHAR NOT NULL DEFAULT 'hot',
			author_type   VARCHAR NOT NULL DEFAULT 'other',
			is_summary    BOOLEAN NOT NULL DEFAULT FALSE,
			summary_of    VARCHAR NOT NULL DEFAULT '[]',
			summary_from  TIMESTAMP,
			summary_to    TIMESTAMP,
			created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			embedding_id  VARCHAR NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_records_ws_ch
			ON records (workspace_id, channel_id);

		CREATE INDEX IF NOT EXISTS idx_records_tier
			ON records (tier);

		CREATE INDEX IF NOT EXISTS idx_records_created
			ON records (created_at);

		CREATE TABLE IF NOT EXISTS channels (
			workspace_id  VARCHAR NOT NULL,
			channel_id    VARCHAR NOT NULL,
			channel_name  VARCHAR NOT NULL,
			is_private    BOOLEAN NOT NULL DEFAULT FALSE,
			num_members   INTEGER NOT NULL DEFAULT 0,
			topic         VARCHAR NOT NULL DEFAULT '',
			purpose       VARCHAR NOT NULL DEFAULT '',
			last_polled   TIMESTAMP,
			last_ts       VARCHAR NOT NULL DEFAULT '',
			cached_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (workspace_id, channel_id)
		);

		CREATE TABLE IF NOT EXISTS users (
			workspace_id  VARCHAR NOT NULL,
			user_id       VARCHAR NOT NULL,
			user_name     VARCHAR NOT NULL DEFAULT '',
			real_name     VARCHAR NOT NULL DEFAULT '',
			cached_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (workspace_id, user_id)
		);
	`
	if _, err := s.db.Exec(ddl); err != nil {
		return err
	}

	// Schema migrations for existing databases.
	// DuckDB ALTER TABLE ADD COLUMN does not support NOT NULL constraints,
	// so columns are added without constraints here.
	migrations := []string{
		`ALTER TABLE channels ADD COLUMN IF NOT EXISTS num_members INTEGER DEFAULT 0`,
		`ALTER TABLE channels ADD COLUMN IF NOT EXISTS cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`,
		`ALTER TABLE records ADD COLUMN IF NOT EXISTS author_type VARCHAR DEFAULT 'other'`,
	}
	for _, m := range migrations {
		s.db.Exec(m) // Ignore errors (column may already exist)
	}

	return nil
}

// InsertRecord inserts a new memory record, skipping if a record with the same ID already exists.
func (s *Store) InsertRecord(ctx context.Context, r *Record) error {
	// Check for existing record to prevent duplicates (e.g., after restart)
	var count int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records WHERE id = ?`, r.ID).Scan(&count)
	if count > 0 {
		return nil // Already exists, skip
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO records (id, workspace_id, workspace_name, channel_id, channel_name,
			user_id, user_name, ts, thread_ts, content, tier, author_type, is_summary,
			summary_of, summary_from, summary_to, created_at, embedding_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.WorkspaceID, r.WorkspaceName, r.ChannelID, r.ChannelName,
		r.UserID, r.UserName, r.Ts, r.ThreadTs, r.Content, string(r.Tier), string(r.AuthorType),
		r.IsSummary, marshalSummaryOf(r.SummaryOf), nullTime(r.SummaryFrom), nullTime(r.SummaryTo),
		r.CreatedAt, r.EmbeddingID,
	)
	return err
}

// FindByChannel returns records for a specific channel, ordered by creation time.
func (s *Store) FindByChannel(ctx context.Context, workspaceID, channelID string, tier Tier, limit int) ([]Record, error) {
	query := `
		SELECT id, workspace_id, workspace_name, channel_id, channel_name,
			user_id, user_name, ts, thread_ts, content, tier, author_type, is_summary,
			summary_of, created_at, embedding_id
		FROM records
		WHERE workspace_id = ? AND channel_id = ? AND tier = ?
		ORDER BY created_at DESC
		LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, workspaceID, channelID, string(tier), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRecords(rows)
}

// FindHotOlderThan returns hot records older than the given duration.
func (s *Store) FindHotOlderThan(ctx context.Context, age time.Duration) ([]Record, error) {
	cutoff := time.Now().Add(-age)

	query := `
		SELECT id, workspace_id, workspace_name, channel_id, channel_name,
			user_id, user_name, ts, thread_ts, content, tier, author_type, is_summary,
			summary_of, created_at, embedding_id
		FROM records
		WHERE tier = 'hot' AND created_at < ?
		ORDER BY workspace_id, channel_id, created_at`

	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRecords(rows)
}

// TransitionTier updates the tier of a record.
func (s *Store) TransitionTier(ctx context.Context, id string, newTier Tier) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE records SET tier = ? WHERE id = ?`,
		string(newTier), id)
	return err
}

// DeleteRecords deletes records by IDs.
func (s *Store) DeleteRecords(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	// Build parameterized query since DuckDB ANY() binding is unreliable
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "DELETE FROM records WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// CountByTier returns the number of records per tier.
func (s *Store) CountByTier(ctx context.Context) (map[Tier]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tier, COUNT(*) FROM records GROUP BY tier`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[Tier]int)
	for rows.Next() {
		var tier string
		var count int
		if err := rows.Scan(&tier, &count); err != nil {
			return nil, err
		}
		counts[Tier(tier)] = count
	}
	return counts, rows.Err()
}

// ChannelStats holds per-channel statistics for the dashboard.
type ChannelStats struct {
	WorkspaceID string
	ChannelID   string
	ChannelName string
	MsgCount    int
	LastTs      string
	LastPolled  *time.Time
}

// GetChannelStats returns per-channel message counts and last activity for a workspace.
func (s *Store) GetChannelStats(ctx context.Context, workspaceID string) ([]ChannelStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.workspace_id, c.channel_id, c.channel_name,
			COALESCE(r.cnt, 0) AS msg_count,
			COALESCE(c.last_ts, '') AS last_ts,
			c.last_polled
		FROM channels c
		LEFT JOIN (
			SELECT workspace_id, channel_id, COUNT(*) AS cnt
			FROM records
			WHERE workspace_id = ?
			GROUP BY workspace_id, channel_id
		) r ON c.workspace_id = r.workspace_id AND c.channel_id = r.channel_id
		WHERE c.workspace_id = ?
		ORDER BY msg_count DESC`,
		workspaceID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChannelStats
	for rows.Next() {
		var cs ChannelStats
		if err := rows.Scan(&cs.WorkspaceID, &cs.ChannelID, &cs.ChannelName, &cs.MsgCount, &cs.LastTs, &cs.LastPolled); err != nil {
			return nil, err
		}
		result = append(result, cs)
	}
	return result, rows.Err()
}

// UpsertChannel inserts or updates channel metadata with cache timestamp.
func (s *Store) UpsertChannel(ctx context.Context, workspaceID, channelID, channelName string, isPrivate bool, numMembers int, topic, purpose string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO channels (workspace_id, channel_id, channel_name, is_private, num_members, topic, purpose, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, channel_id)
		DO UPDATE SET channel_name = EXCLUDED.channel_name,
			is_private = EXCLUDED.is_private,
			num_members = EXCLUDED.num_members,
			topic = EXCLUDED.topic,
			purpose = EXCLUDED.purpose,
			cached_at = EXCLUDED.cached_at`,
		workspaceID, channelID, channelName, isPrivate, numMembers, topic, purpose, now)
	return err
}

// UpdateChannelPolled updates the last polled time and timestamp for a channel.
func (s *Store) UpdateChannelPolled(ctx context.Context, workspaceID, channelID, lastTs string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE channels SET last_polled = CURRENT_TIMESTAMP, last_ts = ?
		WHERE workspace_id = ? AND channel_id = ?`,
		lastTs, workspaceID, channelID)
	return err
}

// CachedChannel holds cached channel data.
type CachedChannel struct {
	WorkspaceID string
	ChannelID   string
	ChannelName string
	IsPrivate   bool
	NumMembers  int
	Topic       string
	Purpose     string
	CachedAt    time.Time
}

// ListCachedChannels returns channels from cache for a workspace.
func (s *Store) ListCachedChannels(ctx context.Context, workspaceID string) ([]CachedChannel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT workspace_id, channel_id, channel_name, is_private, num_members, topic, purpose, cached_at
		FROM channels WHERE workspace_id = ? ORDER BY channel_name`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CachedChannel
	for rows.Next() {
		var ch CachedChannel
		if err := rows.Scan(&ch.WorkspaceID, &ch.ChannelID, &ch.ChannelName, &ch.IsPrivate, &ch.NumMembers, &ch.Topic, &ch.Purpose, &ch.CachedAt); err != nil {
			return nil, err
		}
		result = append(result, ch)
	}
	return result, rows.Err()
}

// ChannelCacheAge returns the oldest cache timestamp for a workspace's channels.
// Returns zero time if no cache exists.
func (s *Store) ChannelCacheAge(ctx context.Context, workspaceID string) (time.Time, error) {
	var oldest time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT MIN(cached_at) FROM channels WHERE workspace_id = ?`,
		workspaceID).Scan(&oldest)
	if err != nil {
		return time.Time{}, nil
	}
	return oldest, nil
}

// UpsertUser inserts or updates user cache.
func (s *Store) UpsertUser(ctx context.Context, workspaceID, userID, userName, realName string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (workspace_id, user_id, user_name, real_name, cached_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, user_id)
		DO UPDATE SET user_name = EXCLUDED.user_name,
			real_name = EXCLUDED.real_name,
			cached_at = EXCLUDED.cached_at`,
		workspaceID, userID, userName, realName, now)
	return err
}

// GetCachedUserName returns the cached user name, or empty string if not cached.
func (s *Store) GetCachedUserName(ctx context.Context, workspaceID, userID string) string {
	var name string
	s.db.QueryRowContext(ctx,
		`SELECT COALESCE(real_name, user_name) FROM users WHERE workspace_id = ? AND user_id = ?`,
		workspaceID, userID).Scan(&name)
	return name
}

func scanRecords(rows *sql.Rows) ([]Record, error) {
	var records []Record
	for rows.Next() {
		var r Record
		var tier string
		var authorType string
		var summaryOfJSON string

		err := rows.Scan(
			&r.ID, &r.WorkspaceID, &r.WorkspaceName, &r.ChannelID, &r.ChannelName,
			&r.UserID, &r.UserName, &r.Ts, &r.ThreadTs, &r.Content, &tier, &authorType,
			&r.IsSummary, &summaryOfJSON, &r.CreatedAt, &r.EmbeddingID,
		)
		if err != nil {
			return nil, err
		}
		r.Tier = Tier(tier)
		r.AuthorType = AuthorType(authorType)
		r.SummaryOf = unmarshalSummaryOf(summaryOfJSON)
		records = append(records, r)
	}
	return records, rows.Err()
}

func marshalSummaryOf(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func unmarshalSummaryOf(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var ids []string
	json.Unmarshal([]byte(s), &ids)
	return ids
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
