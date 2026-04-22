package mitl

import (
	"context"
	"testing"
	"time"
)

func TestCreateAndApprove(t *testing.T) {
	m := NewManager(30 * time.Second)
	ctx := context.Background()

	p := m.CreateProposal(ctx, "WS1", "workspace-1", "CH1", "general",
		"", "what's the status?", "Here's the current status...")

	if p.State != StatePending {
		t.Errorf("expected pending, got %s", p.State)
	}

	pending := m.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	approved, err := m.Approve(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approved.State != StateApproved {
		t.Errorf("expected approved, got %s", approved.State)
	}
	if approved.ResolvedAt.IsZero() {
		t.Error("expected ResolvedAt to be set")
	}

	// No more pending
	pending = m.GetPending()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after approval, got %d", len(pending))
	}
}

func TestReject(t *testing.T) {
	m := NewManager(30 * time.Second)
	ctx := context.Background()

	p := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch",
		"", "trigger", "draft")

	rejected, err := m.Reject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.State != StateRejected {
		t.Errorf("expected rejected, got %s", rejected.State)
	}
}

func TestMarkPosted(t *testing.T) {
	m := NewManager(30 * time.Second)
	ctx := context.Background()

	p := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch",
		"", "trigger", "draft")

	m.Approve(p.ID)

	if err := m.MarkPosted(p.ID); err != nil {
		t.Fatal(err)
	}

	got, ok := m.Get(p.ID)
	if !ok {
		t.Fatal("proposal not found")
	}
	if got.State != StatePosted {
		t.Errorf("expected posted, got %s", got.State)
	}
}

func TestCannotApproveNonPending(t *testing.T) {
	m := NewManager(30 * time.Second)
	ctx := context.Background()

	p := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch",
		"", "trigger", "draft")

	m.Reject(p.ID)

	_, err := m.Approve(p.ID)
	if err == nil {
		t.Error("expected error when approving rejected proposal")
	}
}

func TestTimeout(t *testing.T) {
	expired := make(chan *Proposal, 1)
	m := NewManager(50 * time.Millisecond)
	m.OnExpire = func(p *Proposal) {
		expired <- p
	}

	ctx := context.Background()
	p := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch",
		"", "trigger", "draft")

	select {
	case exp := <-expired:
		if exp.ID != p.ID {
			t.Errorf("expected proposal %q, got %q", p.ID, exp.ID)
		}
		if exp.State != StateExpired {
			t.Errorf("expected expired, got %s", exp.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for expiry")
	}
}

func TestNotificationCallback(t *testing.T) {
	notified := make(chan *Proposal, 1)
	m := NewManager(30 * time.Second)
	m.OnProposal = func(p *Proposal) {
		notified <- p
	}

	ctx := context.Background()
	m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch",
		"", "trigger", "draft")

	select {
	case p := <-notified:
		if p.State != StatePending {
			t.Errorf("expected pending in notification, got %s", p.State)
		}
	case <-time.After(time.Second):
		t.Fatal("notification not received")
	}
}

func TestCleanResolved(t *testing.T) {
	m := NewManager(30 * time.Second)
	ctx := context.Background()

	p1 := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch", "", "t1", "d1")
	p2 := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch", "", "t2", "d2")
	m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch", "", "t3", "d3") // stays pending

	m.Approve(p1.ID)
	m.Reject(p2.ID)

	// Backdate resolved time
	m.mu.Lock()
	m.proposals[p1.ID].ResolvedAt = time.Now().Add(-2 * time.Hour)
	m.proposals[p2.ID].ResolvedAt = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	cleaned := m.CleanResolved(time.Hour)
	if cleaned != 2 {
		t.Errorf("expected 2 cleaned, got %d", cleaned)
	}

	// Pending proposal should remain
	pending := m.GetPending()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending after clean, got %d", len(pending))
	}
}

func TestGetProposal(t *testing.T) {
	m := NewManager(30 * time.Second)
	ctx := context.Background()

	p := m.CreateProposal(ctx, "WS1", "ws", "CH1", "ch", "", "t", "d")

	got, ok := m.Get(p.ID)
	if !ok {
		t.Fatal("expected to find proposal")
	}
	if got.DraftText != "d" {
		t.Errorf("expected draft 'd', got %q", got.DraftText)
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent ID")
	}
}
