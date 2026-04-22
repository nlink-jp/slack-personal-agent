// Package mitl manages the Man-In-The-Loop proxy response workflow.
// Flow: detect intent → generate draft → notify user → await approval → post or discard.
package mitl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// State represents the lifecycle of a proxy response.
type State string

const (
	StatePending  State = "pending"  // Awaiting user approval
	StateApproved State = "approved" // User approved, ready to post
	StateRejected State = "rejected" // User rejected
	StateExpired  State = "expired"  // Timed out without response
	StatePosted   State = "posted"   // Successfully posted to Slack
)

// Proposal represents a draft proxy response awaiting approval.
type Proposal struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	ChannelID     string    `json:"channel_id"`
	ChannelName   string    `json:"channel_name"`
	ThreadTs      string    `json:"thread_ts,omitempty"`
	TriggerText   string    `json:"trigger_text"`   // Message that triggered the response
	DraftText     string    `json:"draft_text"`     // LLM-generated draft response
	State         State     `json:"state"`
	CreatedAt     time.Time `json:"created_at"`
	ResolvedAt    time.Time `json:"resolved_at,omitempty"`
}

// Manager manages pending proxy response proposals.
type Manager struct {
	mu        sync.Mutex
	proposals map[string]*Proposal
	timeout   time.Duration
	// OnProposal is called when a new proposal is created (for notification).
	OnProposal func(p *Proposal)
	// OnExpire is called when a proposal expires.
	OnExpire func(p *Proposal)
}

// NewManager creates a new MITL manager with the given approval timeout.
func NewManager(timeout time.Duration) *Manager {
	return &Manager{
		proposals: make(map[string]*Proposal),
		timeout:   timeout,
	}
}

// CreateProposal creates a new pending proposal and starts the timeout timer.
func (m *Manager) CreateProposal(ctx context.Context, workspaceID, workspaceName, channelID, channelName, threadTs, triggerText, draftText string) *Proposal {
	p := &Proposal{
		ID:            uuid.New().String(),
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
		ChannelID:     channelID,
		ChannelName:   channelName,
		ThreadTs:      threadTs,
		TriggerText:   triggerText,
		DraftText:     draftText,
		State:         StatePending,
		CreatedAt:     time.Now(),
	}

	m.mu.Lock()
	m.proposals[p.ID] = p
	m.mu.Unlock()

	if m.OnProposal != nil {
		m.OnProposal(p)
	}

	// Start timeout goroutine
	go m.watchTimeout(ctx, p.ID)

	return p
}

// Approve marks a proposal as approved. Returns error if not pending.
func (m *Manager) Approve(id string) (*Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.proposals[id]
	if !ok {
		return nil, fmt.Errorf("proposal %q not found", id)
	}
	if p.State != StatePending {
		return nil, fmt.Errorf("proposal %q is %s, not pending", id, p.State)
	}

	p.State = StateApproved
	p.ResolvedAt = time.Now()
	return p, nil
}

// Reject marks a proposal as rejected. Returns error if not pending.
func (m *Manager) Reject(id string) (*Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.proposals[id]
	if !ok {
		return nil, fmt.Errorf("proposal %q not found", id)
	}
	if p.State != StatePending {
		return nil, fmt.Errorf("proposal %q is %s, not pending", id, p.State)
	}

	p.State = StateRejected
	p.ResolvedAt = time.Now()
	return p, nil
}

// MarkPosted marks an approved proposal as posted.
func (m *Manager) MarkPosted(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.proposals[id]
	if !ok {
		return fmt.Errorf("proposal %q not found", id)
	}
	if p.State != StateApproved {
		return fmt.Errorf("proposal %q is %s, not approved", id, p.State)
	}

	p.State = StatePosted
	return nil
}

// GetPending returns all pending proposals.
func (m *Manager) GetPending() []*Proposal {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Proposal
	for _, p := range m.proposals {
		if p.State == StatePending {
			result = append(result, p)
		}
	}
	return result
}

// Get returns a proposal by ID.
func (m *Manager) Get(id string) (*Proposal, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.proposals[id]
	return p, ok
}

// CleanResolved removes non-pending proposals older than the given age.
func (m *Manager) CleanResolved(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0
	for id, p := range m.proposals {
		if p.State != StatePending && p.ResolvedAt.Before(cutoff) {
			delete(m.proposals, id)
			count++
		}
	}
	return count
}

func (m *Manager) watchTimeout(ctx context.Context, id string) {
	select {
	case <-time.After(m.timeout):
	case <-ctx.Done():
		return
	}

	m.mu.Lock()
	p, ok := m.proposals[id]
	if ok && p.State == StatePending {
		p.State = StateExpired
		p.ResolvedAt = time.Now()
	}
	onExpire := m.OnExpire
	m.mu.Unlock()

	// Call callback outside lock to prevent deadlock
	if ok && p.State == StateExpired && onExpire != nil {
		onExpire(p)
	}
}
