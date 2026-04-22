// Package memory manages the 3-tier message lifecycle and storage.
package memory

import "time"

// Tier represents the lifecycle state of a memory record.
type Tier string

const (
	TierHot  Tier = "hot"  // Recent raw data, full text preserved
	TierWarm Tier = "warm" // LLM-summarized, time range retained
	TierCold Tier = "cold" // Archive, read-only
)

// Record represents a stored message or summary in the memory system.
type Record struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	ChannelID     string    `json:"channel_id"`
	ChannelName   string    `json:"channel_name"`
	UserID        string    `json:"user_id"`
	UserName      string    `json:"user_name"`
	Ts            string    `json:"ts"`       // Slack message timestamp
	ThreadTs      string    `json:"thread_ts,omitempty"`
	Content       string    `json:"content"`
	Tier          Tier      `json:"tier"`
	IsSummary     bool      `json:"is_summary"`
	SummaryOf     []string  `json:"summary_of,omitempty"` // IDs of summarized records
	SummaryFrom   time.Time `json:"summary_from,omitempty"`
	SummaryTo     time.Time `json:"summary_to,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	EmbeddingID   string    `json:"embedding_id,omitempty"`
}

// SlackTimestamp returns the Slack timestamp as a time.Time.
// Slack timestamps are Unix epoch with microsecond fraction (e.g., "1713488400.000100").
func (r *Record) SlackTimestamp() time.Time {
	return ParseSlackTs(r.Ts)
}

// ParseSlackTs converts a Slack timestamp string to time.Time.
func ParseSlackTs(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	// Slack ts format: "1713488400.000100" (seconds.microseconds)
	var sec, usec int64
	for i, c := range ts {
		if c == '.' {
			sec = parseInt64(ts[:i])
			usec = parseInt64(ts[i+1:])
			return time.Unix(sec, usec*1000) // microseconds to nanoseconds
		}
	}
	sec = parseInt64(ts)
	return time.Unix(sec, 0)
}

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	return n
}
