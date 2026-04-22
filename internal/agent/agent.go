// Package agent implements the intelligent message evaluation pipeline.
//
// Pipeline stages:
//  1. Relevance: Is this conversation relevant to the user?
//  2. Action: What should the user do? (nothing / respond / review)
//  3. Response: Generate draft reply or action summary for the user.
package agent

import (
	"context"
	"fmt"

	"github.com/nlink-jp/nlk/guard"
	"github.com/nlink-jp/nlk/strip"
	"github.com/nlink-jp/slack-personal-agent/internal/llm"
	"github.com/nlink-jp/slack-personal-agent/internal/rag"
	"github.com/nlink-jp/slack-personal-agent/internal/timectx"
)

// Verdict represents the pipeline's assessment of a message.
type Verdict string

const (
	VerdictIgnore  Verdict = "ignore"  // Not relevant to the user
	VerdictNote    Verdict = "note"    // Relevant but no action needed
	VerdictRespond Verdict = "respond" // User should respond; draft available
	VerdictReview  Verdict = "review"  // User's judgment needed; context provided
)

// Assessment is the output of the pipeline for a batch of new messages.
type Assessment struct {
	Verdict     Verdict `json:"verdict"`
	WorkspaceID string  `json:"workspace_id"`
	ChannelID   string  `json:"channel_id"`
	ChannelName string  `json:"channel_name"`
	ThreadTs    string  `json:"thread_ts,omitempty"`
	Summary     string  `json:"summary"`       // Situation summary for the user
	DraftReply  string  `json:"draft_reply"`   // Only if Verdict == VerdictRespond
	Reason      string  `json:"reason"`        // Why this verdict was chosen
	TriggerText string  `json:"trigger_text"`  // The message(s) that triggered evaluation
}

// Pipeline evaluates incoming messages and decides what the user should do.
type Pipeline struct {
	backend   llm.Backend
	retriever *rag.Retriever
	userName  string // The authenticated user's display name
	userID    string // The authenticated user's Slack ID
}

// NewPipeline creates a new agent pipeline.
func NewPipeline(backend llm.Backend, retriever *rag.Retriever, userName, userID string) *Pipeline {
	return &Pipeline{
		backend:   backend,
		retriever: retriever,
		userName:  userName,
		userID:    userID,
	}
}

// MessageContext holds the information needed to evaluate a batch of messages.
type MessageContext struct {
	WorkspaceID   string
	WorkspaceName string
	ChannelID     string
	ChannelName   string
	Messages      []MessageInfo // New messages to evaluate
	RecentHistory []MessageInfo // Recent channel context (last N messages before new ones)
}

// MessageInfo holds a single message for evaluation.
type MessageInfo struct {
	User      string
	UserName  string
	Text      string
	Ts        string
	ThreadTs  string
	IsBot     bool
	IsSelf    bool
}

// Evaluate runs the full pipeline on a batch of new messages.
// Returns nil if the messages are not relevant or no action is needed.
func (p *Pipeline) Evaluate(ctx context.Context, mc MessageContext) (*Assessment, error) {
	if p.backend == nil {
		return nil, fmt.Errorf("LLM backend not configured")
	}
	if len(mc.Messages) == 0 {
		return nil, nil
	}

	// Build conversation text for LLM
	tag := guard.NewTag()
	conversationText := formatConversation(mc)
	wrappedConversation, _ := tag.Wrap(conversationText)

	// Stage 1+2: Combined relevance and action evaluation
	timeCtx := timectx.Now()
	evalPrompt := buildEvalPrompt(tag, p.userName, p.userID, timeCtx)

	evalReq := &llm.ChatRequest{
		SystemPrompt: evalPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: wrappedConversation},
		},
		ResponseJSON: true,
	}

	evalResp, err := p.backend.Chat(ctx, evalReq)
	if err != nil {
		return nil, fmt.Errorf("evaluation LLM call: %w", err)
	}

	cleaned := strip.ThinkTags(evalResp.Content)
	eval, err := parseEvalResponse(cleaned)
	if err != nil {
		return nil, fmt.Errorf("parse evaluation: %w", err)
	}

	if eval.Verdict == VerdictIgnore || eval.Verdict == VerdictNote {
		return &Assessment{
			Verdict:     eval.Verdict,
			WorkspaceID: mc.WorkspaceID,
			ChannelID:   mc.ChannelID,
			ChannelName: mc.ChannelName,
			Summary:     eval.Summary,
			Reason:      eval.Reason,
			TriggerText: lastMessageText(mc.Messages),
		}, nil
	}

	// Stage 3: Generate response or action summary
	assessment := &Assessment{
		Verdict:     eval.Verdict,
		WorkspaceID: mc.WorkspaceID,
		ChannelID:   mc.ChannelID,
		ChannelName: mc.ChannelName,
		ThreadTs:    eval.ThreadTs,
		Summary:     eval.Summary,
		Reason:      eval.Reason,
		TriggerText: lastMessageText(mc.Messages),
	}

	if eval.Verdict == VerdictRespond {
		// Fetch RAG context for informed response
		scope := rag.SearchScope{
			WorkspaceID: mc.WorkspaceID,
			ChannelID:   mc.ChannelID,
		}
		ragResults, _ := p.retriever.Search(ctx, eval.Summary, scope, 5)

		ragContext := formatRAGResults(ragResults, tag)
		draft, err := p.generateDraft(ctx, tag, timeCtx, wrappedConversation, ragContext, eval.Summary)
		if err != nil {
			// Fall back to review if draft generation fails
			assessment.Verdict = VerdictReview
			assessment.Summary = eval.Summary + "\n\n(Draft generation failed: " + err.Error() + ")"
		} else {
			assessment.DraftReply = draft
		}
	}

	return assessment, nil
}

// generateDraft creates a draft reply using RAG context.
func (p *Pipeline) generateDraft(ctx context.Context, tag guard.Tag, timeCtx, conversation, ragContext, situation string) (string, error) {
	prompt := buildDraftPrompt(tag, p.userName, timeCtx, ragContext)

	req := &llm.ChatRequest{
		SystemPrompt: prompt,
		Messages: []llm.Message{
			{Role: "user", Content: fmt.Sprintf("Conversation:\n%s\n\nSituation: %s\n\nWrite a reply as %s.", conversation, situation, p.userName)},
		},
	}

	resp, err := p.backend.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return strip.ThinkTags(resp.Content), nil
}

func lastMessageText(msgs []MessageInfo) string {
	if len(msgs) == 0 {
		return ""
	}
	last := msgs[len(msgs)-1]
	text := last.Text
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return text
}
