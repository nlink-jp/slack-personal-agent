package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeRecord(wsID, chID, content string, tier Tier, createdAt time.Time) *Record {
	return &Record{
		ID:            uuid.New().String(),
		WorkspaceID:   wsID,
		WorkspaceName: "test-ws",
		ChannelID:     chID,
		ChannelName:   "test-channel",
		UserID:        "U123",
		UserName:      "testuser",
		Content:       content,
		Tier:          tier,
		CreatedAt:     createdAt,
	}
}

func TestInsertAndFind(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	now := time.Now()
	r := makeRecord("WS1", "CH1", "hello world", TierHot, now)

	if err := s.InsertRecord(ctx, r); err != nil {
		t.Fatal(err)
	}

	records, err := s.FindByChannel(ctx, "WS1", "CH1", TierHot, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", records[0].Content)
	}
	if records[0].Tier != TierHot {
		t.Errorf("expected TierHot, got %q", records[0].Tier)
	}
}

func TestFindByChannelIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()

	// Insert records in different workspaces/channels
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "ws1-ch1", TierHot, now))
	s.InsertRecord(ctx, makeRecord("WS1", "CH2", "ws1-ch2", TierHot, now))
	s.InsertRecord(ctx, makeRecord("WS2", "CH1", "ws2-ch1", TierHot, now))

	// Find should only return matching workspace+channel
	records, err := s.FindByChannel(ctx, "WS1", "CH1", TierHot, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Content != "ws1-ch1" {
		t.Errorf("expected 'ws1-ch1', got %q", records[0].Content)
	}
}

func TestCountByTier(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()

	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "hot1", TierHot, now))
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "hot2", TierHot, now))
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "warm1", TierWarm, now))

	counts, err := s.CountByTier(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if counts[TierHot] != 2 {
		t.Errorf("expected 2 hot, got %d", counts[TierHot])
	}
	if counts[TierWarm] != 1 {
		t.Errorf("expected 1 warm, got %d", counts[TierWarm])
	}
}

func TestTransitionTier(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r := makeRecord("WS1", "CH1", "test", TierHot, time.Now())
	s.InsertRecord(ctx, r)

	if err := s.TransitionTier(ctx, r.ID, TierWarm); err != nil {
		t.Fatal(err)
	}

	hot, _ := s.FindByChannel(ctx, "WS1", "CH1", TierHot, 10)
	warm, _ := s.FindByChannel(ctx, "WS1", "CH1", TierWarm, 10)

	if len(hot) != 0 {
		t.Errorf("expected 0 hot records, got %d", len(hot))
	}
	if len(warm) != 1 {
		t.Errorf("expected 1 warm record, got %d", len(warm))
	}
}

func TestFindHotOlderThan(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)

	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "old", TierHot, old))
	s.InsertRecord(ctx, makeRecord("WS1", "CH1", "recent", TierHot, recent))

	records, err := s.FindHotOlderThan(ctx, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 old record, got %d", len(records))
	}
	if records[0].Content != "old" {
		t.Errorf("expected 'old', got %q", records[0].Content)
	}
}

func TestUpsertChannel(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	err := s.UpsertChannel(ctx, "WS1", "CH1", "general", false, 42, "topic", "purpose")
	if err != nil {
		t.Fatal(err)
	}

	// Update same channel
	err = s.UpsertChannel(ctx, "WS1", "CH1", "general-renamed", false, 42, "new topic", "new purpose")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRecords(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	r1 := makeRecord("WS1", "CH1", "one", TierHot, time.Now())
	r2 := makeRecord("WS1", "CH1", "two", TierHot, time.Now())
	s.InsertRecord(ctx, r1)
	s.InsertRecord(ctx, r2)

	if err := s.DeleteRecords(ctx, []string{r1.ID}); err != nil {
		t.Fatal(err)
	}

	records, _ := s.FindByChannel(ctx, "WS1", "CH1", TierHot, 10)
	if len(records) != 1 {
		t.Fatalf("expected 1 record after delete, got %d", len(records))
	}
	if records[0].ID != r2.ID {
		t.Errorf("expected r2 to remain, got %q", records[0].ID)
	}
}
