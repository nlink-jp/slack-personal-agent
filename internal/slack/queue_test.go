package slack

import (
	"context"
	"testing"
	"time"
)

func TestQueuePriority(t *testing.T) {
	q := NewQueue(50)

	q.Enqueue("ch-normal")
	q.Boost("ch-boosted", 5*time.Minute)
	q.Enqueue("ch-boosted")

	ctx := context.Background()

	// Boosted channel should come first
	entry := q.Dequeue(ctx)
	if entry.ChannelID != "ch-boosted" {
		t.Errorf("expected boosted channel first, got %q", entry.ChannelID)
	}
	if entry.Priority != PriorityHigh {
		t.Errorf("expected PriorityHigh, got %d", entry.Priority)
	}

	entry = q.Dequeue(ctx)
	if entry.ChannelID != "ch-normal" {
		t.Errorf("expected normal channel second, got %q", entry.ChannelID)
	}
}

func TestQueueFIFOSamePriority(t *testing.T) {
	q := NewQueue(50)

	q.Enqueue("ch-1")
	time.Sleep(time.Millisecond)
	q.Enqueue("ch-2")
	time.Sleep(time.Millisecond)
	q.Enqueue("ch-3")

	ctx := context.Background()

	ids := make([]string, 3)
	for i := range ids {
		entry := q.Dequeue(ctx)
		ids[i] = entry.ChannelID
	}

	if ids[0] != "ch-1" || ids[1] != "ch-2" || ids[2] != "ch-3" {
		t.Errorf("expected FIFO order, got %v", ids)
	}
}

func TestQueueRateLimitCanCall(t *testing.T) {
	q := NewQueue(2) // Very low limit for testing

	// Simulate 2 recent calls within the last minute
	q.mu.Lock()
	now := time.Now()
	q.callTimes = []time.Time{now.Add(-30 * time.Second), now.Add(-15 * time.Second)}
	q.mu.Unlock()

	q.mu.Lock()
	canCall := q.canCall()
	q.mu.Unlock()

	if canCall {
		t.Error("expected canCall=false when at rate limit")
	}

	// Simulate calls older than 1 minute (should be pruned)
	q.mu.Lock()
	q.callTimes = []time.Time{now.Add(-90 * time.Second), now.Add(-80 * time.Second)}
	q.mu.Unlock()

	q.mu.Lock()
	canCall = q.canCall()
	q.mu.Unlock()

	if !canCall {
		t.Error("expected canCall=true after old calls pruned")
	}
}

func TestQueueLen(t *testing.T) {
	q := NewQueue(50)

	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d", q.Len())
	}

	q.Enqueue("ch-1")
	q.Enqueue("ch-2")

	if q.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", q.Len())
	}
}

func TestQueueBoostExpiry(t *testing.T) {
	q := NewQueue(50)

	// Boost with very short duration
	q.Boost("ch-1", time.Millisecond)
	time.Sleep(5 * time.Millisecond) // Wait for expiry

	q.Enqueue("ch-1")

	ctx := context.Background()
	entry := q.Dequeue(ctx)

	// Boost should have expired, so priority should be normal
	if entry.Priority != PriorityNormal {
		t.Errorf("expected PriorityNormal after boost expiry, got %d", entry.Priority)
	}
}

func TestQueueBoostedChannels(t *testing.T) {
	q := NewQueue(50)

	q.Boost("ch-1", 5*time.Minute)
	q.Boost("ch-2", 5*time.Minute)
	q.Boost("ch-expired", time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	boosted := q.BoostedChannels()

	if len(boosted) != 2 {
		t.Errorf("expected 2 boosted channels, got %d: %v", len(boosted), boosted)
	}

	found := map[string]bool{}
	for _, id := range boosted {
		found[id] = true
	}
	if !found["ch-1"] || !found["ch-2"] {
		t.Errorf("expected ch-1 and ch-2, got %v", boosted)
	}
}

func TestQueueContextCancellation(t *testing.T) {
	q := NewQueue(50)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	entry := q.Dequeue(ctx)
	if entry != nil {
		t.Error("expected nil from cancelled context")
	}
}

func TestSchedulerSetChannels(t *testing.T) {
	q := NewQueue(50)
	client := NewClient("test-token")
	s := NewScheduler(client, q, time.Minute, 30*time.Second)

	s.SetChannels([]string{"ch-1", "ch-2", "ch-3"})

	if len(s.channels) != 3 {
		t.Errorf("expected 3 channels, got %d", len(s.channels))
	}
}
