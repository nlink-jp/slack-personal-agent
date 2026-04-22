package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
	"github.com/nlink-jp/slack-personal-agent/internal/embedding"
	"github.com/nlink-jp/slack-personal-agent/internal/keychain"
	"github.com/nlink-jp/slack-personal-agent/internal/llm"
	"github.com/nlink-jp/slack-personal-agent/internal/memory"
	"github.com/nlink-jp/slack-personal-agent/internal/mitl"
	"github.com/nlink-jp/slack-personal-agent/internal/rag"
	"github.com/nlink-jp/slack-personal-agent/internal/slack"
)

// App holds the application state and provides Wails bindings.
type App struct {
	ctx       context.Context
	cfg       *config.Config
	store     *memory.Store
	retriever *rag.Retriever
	backend   llm.Backend
	embedder  embedding.Embedder
	keys      keychain.Store
	mitlMgr   *mitl.Manager
	clients   map[string]*slack.Client // workspace → client for posting
	pollers   map[string]*slack.WorkspacePoller
	mu        sync.Mutex
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{
		pollers: make(map[string]*slack.WorkspacePoller),
		clients: make(map[string]*slack.Client),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load config
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		log.Printf("Warning: config load failed: %v", err)
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg

	// Initialize keychain
	a.keys = &keychain.OSStore{}

	// Initialize memory store (DuckDB)
	dataDir := config.DefaultDataDir()
	dbPath := filepath.Join(dataDir, "spa.db")
	store, err := memory.Open(dbPath)
	if err != nil {
		log.Printf("Error: failed to open database: %v", err)
		return
	}
	a.store = store

	// Initialize LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		log.Printf("Warning: LLM backend init failed: %v", err)
	}
	a.backend = backend

	// Initialize embedding
	embedder, err := embedding.NewEmbedder(cfg)
	if err != nil {
		log.Printf("Warning: embedding init failed (using mock): %v", err)
		embedder = embedding.NewMockEmbedder(384)
	}
	a.embedder = embedder

	// Initialize RAG retriever
	retriever := rag.NewRetriever(store.DB(), embedder)
	if err := retriever.Migrate(); err != nil {
		log.Printf("Error: RAG migration failed: %v", err)
	}
	a.retriever = retriever

	// Initialize MITL manager
	a.mitlMgr = mitl.NewManager(cfg.Response.Timeout())
	a.mitlMgr.OnProposal = func(p *mitl.Proposal) {
		log.Printf("MITL proposal: [%s/%s] %s", p.WorkspaceName, p.ChannelName, p.DraftText[:min(len(p.DraftText), 80)])
	}
	a.mitlMgr.OnExpire = func(p *mitl.Proposal) {
		log.Printf("MITL expired: %s", p.ID)
	}

	// Check embedding model consistency
	storedID, consistent, err := retriever.CheckModelConsistency(ctx)
	if err != nil {
		log.Printf("Warning: model consistency check failed: %v", err)
	} else if !consistent {
		log.Printf("Warning: embedding model changed (stored=%q, current=%q) — re-index recommended", storedID, embedder.ModelID())
	}
}

// shutdown is called when the app is closing.
func (a *App) shutdown(_ context.Context) {
	if a.store != nil {
		a.store.Close()
	}
}

// ── Wails Bindings ──────────────────────────────────────

// Version returns the application version.
func (a *App) Version() string {
	return version
}

// GetConfig returns the current configuration (without secrets).
func (a *App) GetConfig() *config.Config {
	return a.cfg
}

// SaveConfig saves the configuration to disk.
func (a *App) SaveConfig(cfg *config.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := config.Save(cfg, config.DefaultConfigPath()); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.cfg = cfg
	return nil
}

// GetWorkspaces returns the list of configured workspaces with connection status.
func (a *App) GetWorkspaces() []WorkspaceStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	var result []WorkspaceStatus
	for _, ws := range a.cfg.Workspaces {
		hasToken := false
		if _, err := a.keys.Get(keychain.WorkspaceTokenKey(ws.Name)); err == nil {
			hasToken = true
		}
		_, polling := a.pollers[ws.Name]
		result = append(result, WorkspaceStatus{
			Name:     ws.Name,
			HasToken: hasToken,
			Polling:  polling,
		})
	}
	return result
}

// WorkspaceStatus holds workspace information for the frontend.
type WorkspaceStatus struct {
	Name     string `json:"name"`
	HasToken bool   `json:"has_token"`
	Polling  bool   `json:"polling"`
}

// SetWorkspaceToken stores a workspace token in the keychain.
func (a *App) SetWorkspaceToken(workspace, token string) error {
	return a.keys.Set(keychain.WorkspaceTokenKey(workspace), token)
}

// StartPolling starts polling for a workspace.
func (a *App) StartPolling(workspace string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.pollers[workspace]; exists {
		return fmt.Errorf("already polling workspace %q", workspace)
	}

	token, err := a.keys.Get(keychain.WorkspaceTokenKey(workspace))
	if err != nil {
		return fmt.Errorf("no token for workspace %q: %w", workspace, err)
	}

	client := slack.NewClient(token)
	queue := slack.NewQueue(a.cfg.Polling.MaxRatePerMin)
	scheduler := slack.NewScheduler(client, queue,
		a.cfg.Polling.Interval(),
		a.cfg.Polling.PriorityBoostInterval())

	// Discover channels
	channels, err := client.ListChannels(a.ctx)
	if err != nil {
		return fmt.Errorf("list channels for %q: %w", workspace, err)
	}

	channelIDs := make([]string, 0, len(channels))
	for _, ch := range channels {
		channelIDs = append(channelIDs, ch.ID)
		a.store.UpsertChannel(a.ctx, workspace, ch.ID, ch.Name, ch.IsPrivate, ch.Topic.Value, ch.Purpose.Value)
	}
	scheduler.SetChannels(channelIDs)

	poller := slack.NewWorkspacePoller(workspace, client, queue, scheduler)
	poller.OnMessages = a.handleMessages

	a.pollers[workspace] = poller
	a.clients[workspace] = client

	// Start scheduler and poller in background
	go scheduler.Run(a.ctx)
	go poller.Run(a.ctx)

	log.Printf("Started polling %q (%d channels)", workspace, len(channelIDs))
	return nil
}

// StopPolling stops polling for a workspace.
func (a *App) StopPolling(workspace string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.pollers[workspace]; !exists {
		return fmt.Errorf("not polling workspace %q", workspace)
	}
	// Poller will stop when context is cancelled
	delete(a.pollers, workspace)
	return nil
}

// Query performs a channel-scoped RAG query.
func (a *App) Query(workspaceID, channelID, question string) ([]QueryResult, error) {
	scope := rag.SearchScope{
		WorkspaceID: workspaceID,
		ChannelID:   channelID,
	}

	results, err := a.retriever.Search(a.ctx, question, scope, 10)
	if err != nil {
		return nil, err
	}

	var out []QueryResult
	for _, r := range results {
		out = append(out, QueryResult{
			RecordID:    r.RecordID,
			WorkspaceID: r.WorkspaceID,
			ChannelID:   r.ChannelID,
			Score:       r.Score,
		})
	}
	return out, nil
}

// QueryResult is the frontend-facing search result.
type QueryResult struct {
	RecordID    string  `json:"record_id"`
	WorkspaceID string  `json:"workspace_id"`
	ChannelID   string  `json:"channel_id"`
	Score       float64 `json:"score"`
}

// GetMemoryStats returns the current memory tier counts.
func (a *App) GetMemoryStats() (map[string]int, error) {
	counts, err := a.store.CountByTier(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for tier, count := range counts {
		result[string(tier)] = count
	}
	return result, nil
}

// ── MITL Proxy Response ─────────────────────────────────

// GetPendingProposals returns all pending MITL proposals.
func (a *App) GetPendingProposals() []*mitl.Proposal {
	return a.mitlMgr.GetPending()
}

// ApproveProposal approves a MITL proposal and posts it to Slack.
func (a *App) ApproveProposal(id string) error {
	p, err := a.mitlMgr.Approve(id)
	if err != nil {
		return err
	}

	a.mu.Lock()
	client, ok := a.clients[p.WorkspaceID]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("no client for workspace %q", p.WorkspaceID)
	}

	// Post with signature for sender identification
	_, err = client.PostProxyMessage(a.ctx, p.ChannelID, p.DraftText, p.ThreadTs, a.cfg.Response.Signature)
	if err != nil {
		return fmt.Errorf("post proxy message: %w", err)
	}

	// Boost polling priority for this channel
	a.mu.Lock()
	if poller, exists := a.pollers[p.WorkspaceID]; exists {
		poller.Scheduler.BoostChannel(p.ChannelID)
	}
	a.mu.Unlock()

	return a.mitlMgr.MarkPosted(id)
}

// RejectProposal rejects a MITL proposal.
func (a *App) RejectProposal(id string) error {
	_, err := a.mitlMgr.Reject(id)
	return err
}

// EditAndApproveProposal allows the user to edit the draft before approving.
func (a *App) EditAndApproveProposal(id, editedText string) error {
	a.mu.Lock()
	p, ok := a.mitlMgr.Get(id)
	if ok {
		p.DraftText = editedText
	}
	a.mu.Unlock()

	if !ok {
		return fmt.Errorf("proposal %q not found", id)
	}
	return a.ApproveProposal(id)
}

// handleMessages processes new messages from a workspace poller.
func (a *App) handleMessages(workspaceName, channelID string, messages []slack.Message) {
	for _, msg := range messages {
		if msg.SubType != "" {
			continue // Skip system messages
		}

		record := &memory.Record{
			ID:            fmt.Sprintf("%s-%s-%s", workspaceName, channelID, msg.Ts),
			WorkspaceID:   workspaceName,
			WorkspaceName: workspaceName,
			ChannelID:     channelID,
			UserID:        msg.User,
			Ts:            msg.Ts,
			ThreadTs:      msg.ThreadTs,
			Content:       msg.Text,
			Tier:          memory.TierHot,
			CreatedAt:     time.Now(),
		}

		if err := a.store.InsertRecord(a.ctx, record); err != nil {
			log.Printf("Error inserting record: %v", err)
			continue
		}

		// Index for RAG
		embID := record.ID + "-emb"
		if err := a.retriever.Index(a.ctx, embID, record.ID, workspaceName, channelID, msg.Text); err != nil {
			log.Printf("Error indexing record: %v", err)
		}
	}
}
