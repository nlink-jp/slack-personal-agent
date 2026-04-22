package slack

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// Priority levels for API queue entries.
const (
	PriorityHigh   = 0 // Boosted channels (active response)
	PriorityNormal = 1 // Regular polling
)

// QueueEntry represents a pending API call in the priority queue.
type QueueEntry struct {
	ChannelID string
	Priority  int
	ScheduledAt time.Time
	index     int // heap internal
}

// Queue manages rate-limited Slack API calls for a single workspace.
// It enforces a maximum request rate and supports priority boosting.
type Queue struct {
	mu          sync.Mutex
	entries     entryHeap
	maxPerMin   int
	callTimes   []time.Time // sliding window of recent API call times
	boosted     map[string]time.Time // channelID → boost expiry
}

// NewQueue creates a new rate-limited API queue.
// maxPerMin is the maximum number of API calls per minute (Slack Tier 3 ≈ 50).
func NewQueue(maxPerMin int) *Queue {
	return &Queue{
		maxPerMin: maxPerMin,
		boosted:   make(map[string]time.Time),
	}
}

// Enqueue adds a channel poll request to the queue.
func (q *Queue) Enqueue(channelID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	priority := PriorityNormal
	if expiry, ok := q.boosted[channelID]; ok {
		if time.Now().Before(expiry) {
			priority = PriorityHigh
		} else {
			delete(q.boosted, channelID)
		}
	}

	entry := &QueueEntry{
		ChannelID:   channelID,
		Priority:    priority,
		ScheduledAt: time.Now(),
	}
	heap.Push(&q.entries, entry)
}

// Boost elevates a channel's priority for the given duration.
func (q *Queue) Boost(channelID string, duration time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.boosted[channelID] = time.Now().Add(duration)
}

// Dequeue returns the next entry to process, blocking until rate limit allows.
// Returns nil if the context is cancelled.
func (q *Queue) Dequeue(ctx context.Context) *QueueEntry {
	for {
		q.mu.Lock()
		if q.entries.Len() == 0 {
			q.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		if !q.canCall() {
			q.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(q.waitTime()):
				continue
			}
		}

		entry := heap.Pop(&q.entries).(*QueueEntry)
		q.recordCall()
		q.mu.Unlock()
		return entry
	}
}

// Len returns the number of pending entries.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.entries.Len()
}

// BoostedChannels returns the IDs of currently boosted channels.
func (q *Queue) BoostedChannels() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	var result []string
	for id, expiry := range q.boosted {
		if now.Before(expiry) {
			result = append(result, id)
		}
	}
	return result
}

// canCall checks if we're within the rate limit. Must be called with mu held.
func (q *Queue) canCall() bool {
	q.pruneCallTimes()
	return len(q.callTimes) < q.maxPerMin
}

// waitTime returns how long to wait before the next call is allowed. Must be called with mu held.
func (q *Queue) waitTime() time.Duration {
	q.pruneCallTimes()
	if len(q.callTimes) == 0 {
		return 0
	}
	oldest := q.callTimes[0]
	wait := oldest.Add(time.Minute).Sub(time.Now())
	if wait < 0 {
		return 0
	}
	return wait
}

// recordCall adds the current time to the call window. Must be called with mu held.
func (q *Queue) recordCall() {
	q.callTimes = append(q.callTimes, time.Now())
}

// pruneCallTimes removes entries older than 1 minute. Must be called with mu held.
func (q *Queue) pruneCallTimes() {
	cutoff := time.Now().Add(-time.Minute)
	i := 0
	for i < len(q.callTimes) && q.callTimes[i].Before(cutoff) {
		i++
	}
	q.callTimes = q.callTimes[i:]
}

// entryHeap implements heap.Interface for priority queue ordering.
// Lower priority number = higher priority. Ties broken by ScheduledAt.
type entryHeap []*QueueEntry

func (h entryHeap) Len() int { return len(h) }

func (h entryHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].ScheduledAt.Before(h[j].ScheduledAt)
}

func (h entryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *entryHeap) Push(x interface{}) {
	entry := x.(*QueueEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}

func (h *entryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*h = old[:n-1]
	return entry
}

// Scheduler manages periodic polling across channels in a workspace.
type Scheduler struct {
	queue    *Queue
	client   *Client
	channels []string
	chMu     sync.Mutex
	interval time.Duration
	boostDur time.Duration
}

// NewScheduler creates a polling scheduler for the given workspace.
func NewScheduler(client *Client, queue *Queue, interval, boostDuration time.Duration) *Scheduler {
	return &Scheduler{
		queue:    queue,
		client:   client,
		interval: interval,
		boostDur: boostDuration,
	}
}

// SetChannels updates the list of channels to poll.
func (s *Scheduler) SetChannels(channelIDs []string) {
	s.chMu.Lock()
	defer s.chMu.Unlock()
	s.channels = channelIDs
}

// Run starts the polling loop. It enqueues all channels at the configured
// interval. Call cancel on the context to stop.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Initial enqueue
	s.enqueueAll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.enqueueAll()
		}
	}
}

// BoostChannel temporarily increases the polling priority for a channel.
func (s *Scheduler) BoostChannel(channelID string) {
	s.queue.Boost(channelID, s.boostDur)
}

func (s *Scheduler) enqueueAll() {
	s.chMu.Lock()
	chs := make([]string, len(s.channels))
	copy(chs, s.channels)
	s.chMu.Unlock()

	for _, ch := range chs {
		s.queue.Enqueue(ch)
	}
}

// WorkspacePoller coordinates polling and message processing for a workspace.
type WorkspacePoller struct {
	Name      string
	Client    *Client
	Queue     *Queue
	Scheduler *Scheduler
	// OnMessages is called when new messages are fetched for a channel.
	OnMessages func(workspaceName, channelID string, messages []Message)
	// lastTs tracks the latest timestamp per channel for incremental polling.
	lastTs map[string]string
	mu     sync.Mutex
}

// NewWorkspacePoller creates a poller for a single workspace.
func NewWorkspacePoller(name string, client *Client, queue *Queue, scheduler *Scheduler) *WorkspacePoller {
	return &WorkspacePoller{
		Name:      name,
		Client:    client,
		Queue:     queue,
		Scheduler: scheduler,
		lastTs:    make(map[string]string),
	}
}

// Run starts the workspace poller. It processes queue entries and fetches messages.
func (wp *WorkspacePoller) Run(ctx context.Context) {
	for {
		entry := wp.Queue.Dequeue(ctx)
		if entry == nil {
			return // context cancelled
		}

		wp.mu.Lock()
		oldest := wp.lastTs[entry.ChannelID]
		wp.mu.Unlock()

		messages, err := wp.Client.FetchHistory(ctx, entry.ChannelID, oldest, 200)
		if err != nil {
			fmt.Printf("[%s] error fetching %s: %v\n", wp.Name, entry.ChannelID, err)
			continue
		}

		if len(messages) > 0 {
			// Messages come newest-first; update lastTs with the newest
			wp.mu.Lock()
			wp.lastTs[entry.ChannelID] = messages[0].Ts
			wp.mu.Unlock()

			if wp.OnMessages != nil {
				wp.OnMessages(wp.Name, entry.ChannelID, messages)
			}
		}
	}
}
