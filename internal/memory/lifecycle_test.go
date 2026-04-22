package memory

import (
	"context"
	"testing"
	"time"
)

func TestLifecycleCompactHotToWarm(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert old hot records (48 hours ago)
	old := time.Now().Add(-48 * time.Hour)
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "old msg 1", TierHot, old))
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "old msg 2", TierHot, old.Add(time.Minute)))

	// Insert recent hot record (1 hour ago)
	recent := time.Now().Add(-1 * time.Hour)
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "recent msg", TierHot, recent))

	lc := NewLifecycle(s, 24*time.Hour, 7*24*time.Hour)
	lc.Summarize = func(_ context.Context, records []Record) (string, error) {
		return "Summary of 2 messages", nil
	}

	if err := lc.RunCompaction(ctx); err != nil {
		t.Fatal(err)
	}

	// Should have 1 hot (recent) + 1 warm (summary)
	hot, _ := s.FindByChannel(ctx, "WS1", "CH1", TierHot, 10)
	warm, _ := s.FindByChannel(ctx, "WS1", "CH1", TierWarm, 10)

	if len(hot) != 1 {
		t.Errorf("expected 1 hot record, got %d", len(hot))
	}
	if hot[0].Content != "recent msg" {
		t.Errorf("expected 'recent msg', got %q", hot[0].Content)
	}

	if len(warm) != 1 {
		t.Errorf("expected 1 warm record, got %d", len(warm))
	}
	if !warm[0].IsSummary {
		t.Error("expected warm record to be a summary")
	}
	if warm[0].Content != "Summary of 2 messages" {
		t.Errorf("expected summary content, got %q", warm[0].Content)
	}
	if len(warm[0].SummaryOf) != 2 {
		t.Errorf("expected SummaryOf length 2, got %d", len(warm[0].SummaryOf))
	}
}

func TestLifecycleWarmToCold(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	old := time.Now().Add(-10 * 24 * time.Hour) // 10 days ago
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "old warm", TierWarm, old))

	recent := time.Now().Add(-1 * time.Hour)
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "recent warm", TierWarm, recent))

	lc := NewLifecycle(s, 24*time.Hour, 7*24*time.Hour)

	if err := lc.RunCompaction(ctx); err != nil {
		t.Fatal(err)
	}

	warm, _ := s.FindByChannel(ctx, "WS1", "CH1", TierWarm, 10)
	cold, _ := s.FindByChannel(ctx, "WS1", "CH1", TierCold, 10)

	if len(warm) != 1 {
		t.Errorf("expected 1 warm record, got %d", len(warm))
	}
	if len(cold) != 1 {
		t.Errorf("expected 1 cold record, got %d", len(cold))
	}
	if cold[0].Content != "old warm" {
		t.Errorf("expected 'old warm', got %q", cold[0].Content)
	}
}

func TestLifecycleChannelIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "ch1 msg", TierHot, old))
	s.InsertRecord(ctx, makeRecord("WS1", "CH2", "ch2 msg", TierHot, old))
	s.InsertRecord(ctx, makeRecord("WS2", "CH1", "ws2 msg", TierHot, old))

	summaryCount := 0
	lc := NewLifecycle(s, 24*time.Hour, 7*24*time.Hour)
	lc.Summarize = func(_ context.Context, records []Record) (string, error) {
		summaryCount++
		return "summary", nil
	}

	if err := lc.RunCompaction(ctx); err != nil {
		t.Fatal(err)
	}

	// Should create 3 separate summaries (one per workspace+channel)
	if summaryCount != 3 {
		t.Errorf("expected 3 summaries (per ws+ch), got %d", summaryCount)
	}
}

func TestLifecycleNoSummarizer(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "msg", TierHot, old))

	lc := NewLifecycle(s, 24*time.Hour, 7*24*time.Hour)
	// No summarizer set

	if err := lc.RunCompaction(ctx); err != nil {
		t.Fatal(err)
	}

	// Without summarizer, records transition directly
	hot, _ := s.FindByChannel(ctx, "WS1", "CH1", TierHot, 10)
	warm, _ := s.FindByChannel(ctx, "WS1", "CH1", TierWarm, 10)

	if len(hot) != 0 {
		t.Errorf("expected 0 hot, got %d", len(hot))
	}
	if len(warm) != 1 {
		t.Errorf("expected 1 warm, got %d", len(warm))
	}
}
