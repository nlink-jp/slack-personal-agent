package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Lifecycle manages the Hot → Warm → Cold record transitions.
type Lifecycle struct {
	store        *Store
	hotToWarm    time.Duration
	warmToCold   time.Duration
	// Summarize is called to generate a summary of records.
	// It receives the records to summarize and returns the summary text.
	Summarize func(ctx context.Context, records []Record) (string, error)
}

// NewLifecycle creates a new lifecycle manager.
func NewLifecycle(store *Store, hotToWarm, warmToCold time.Duration) *Lifecycle {
	return &Lifecycle{
		store:      store,
		hotToWarm:  hotToWarm,
		warmToCold: warmToCold,
	}
}

// RunCompaction performs one compaction cycle:
// 1. Hot records older than hotToWarm → summarize → create Warm record, delete Hot records
// 2. Warm records older than warmToCold → transition to Cold
func (l *Lifecycle) RunCompaction(ctx context.Context) error {
	if err := l.compactHotToWarm(ctx); err != nil {
		return fmt.Errorf("hot→warm: %w", err)
	}
	if err := l.transitionWarmToCold(ctx); err != nil {
		return fmt.Errorf("warm→cold: %w", err)
	}
	return nil
}

func (l *Lifecycle) compactHotToWarm(ctx context.Context) error {
	records, err := l.store.FindHotOlderThan(ctx, l.hotToWarm)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	// Group by workspace+channel
	groups := groupByChannel(records)

	for key, group := range groups {
		if l.Summarize == nil {
			// No summarizer configured; just transition without summarizing
			for _, r := range group {
				if err := l.store.TransitionTier(ctx, r.ID, TierWarm); err != nil {
					return fmt.Errorf("transition %s: %w", r.ID, err)
				}
			}
			continue
		}

		summary, err := l.Summarize(ctx, group)
		if err != nil {
			return fmt.Errorf("summarize %s: %w", key, err)
		}

		// Determine time range
		var earliest, latest time.Time
		ids := make([]string, len(group))
		for i, r := range group {
			ids[i] = r.ID
			t := r.CreatedAt
			if earliest.IsZero() || t.Before(earliest) {
				earliest = t
			}
			if latest.IsZero() || t.After(latest) {
				latest = t
			}
		}

		// Insert warm summary record
		warmRecord := &Record{
			ID:            uuid.New().String(),
			WorkspaceID:   group[0].WorkspaceID,
			WorkspaceName: group[0].WorkspaceName,
			ChannelID:     group[0].ChannelID,
			ChannelName:   group[0].ChannelName,
			Content:       summary,
			Tier:          TierWarm,
			IsSummary:     true,
			SummaryOf:     ids,
			SummaryFrom:   earliest,
			SummaryTo:     latest,
			CreatedAt:     time.Now(),
		}
		if err := l.store.InsertRecord(ctx, warmRecord); err != nil {
			return fmt.Errorf("insert warm record: %w", err)
		}

		// Delete original hot records
		if err := l.store.DeleteRecords(ctx, ids); err != nil {
			return fmt.Errorf("delete hot records: %w", err)
		}
	}
	return nil
}

func (l *Lifecycle) transitionWarmToCold(ctx context.Context) error {
	cutoff := time.Now().Add(-l.warmToCold)

	_, err := l.store.db.ExecContext(ctx, `
		UPDATE records SET tier = 'cold'
		WHERE tier = 'warm' AND created_at < ?`, cutoff)
	return err
}

// channelKey is a composite key for grouping records.
type channelKey struct {
	WorkspaceID string
	ChannelID   string
}

func groupByChannel(records []Record) map[channelKey][]Record {
	groups := make(map[channelKey][]Record)
	for _, r := range records {
		key := channelKey{WorkspaceID: r.WorkspaceID, ChannelID: r.ChannelID}
		groups[key] = append(groups[key], r)
	}
	return groups
}
