package agent

import "testing"

func TestParseEvalResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Verdict
		wantErr bool
	}{
		{
			name:  "respond verdict",
			input: `{"verdict": "respond", "summary": "User was asked about timeline", "reason": "Direct question to user", "thread_ts": "1713488400.000100"}`,
			want:  VerdictRespond,
		},
		{
			name:  "ignore verdict",
			input: `{"verdict": "ignore", "summary": "General chatter", "reason": "Not relevant"}`,
			want:  VerdictIgnore,
		},
		{
			name:  "review verdict",
			input: `{"verdict": "review", "summary": "Budget decision needed", "reason": "Requires human judgment"}`,
			want:  VerdictReview,
		},
		{
			name:  "note verdict",
			input: `{"verdict": "note", "summary": "FYI about deployment", "reason": "Informational only"}`,
			want:  VerdictNote,
		},
		{
			name:  "with markdown fence",
			input: "```json\n{\"verdict\": \"respond\", \"summary\": \"test\", \"reason\": \"test\"}\n```",
			want:  VerdictRespond,
		},
		{
			name:    "invalid verdict",
			input:   `{"verdict": "unknown", "summary": "test", "reason": "test"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   "this is not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEvalResponse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Verdict != tt.want {
				t.Errorf("verdict = %q, want %q", got.Verdict, tt.want)
			}
		})
	}
}

func TestFormatConversation(t *testing.T) {
	mc := MessageContext{
		ChannelName: "general",
		Messages: []MessageInfo{
			{User: "U1", UserName: "alice", Text: "hello", Ts: "100.0"},
			{User: "U2", UserName: "bob", Text: "hi there", Ts: "101.0", IsBot: true},
		},
	}

	text := formatConversation(mc)

	if !contains(text, "#general") {
		t.Error("expected channel name in output")
	}
	if !contains(text, "alice: hello") {
		t.Error("expected alice's message")
	}
	if !contains(text, "[bot]") {
		t.Error("expected bot tag for bob")
	}
}

func TestFormatConversationWithSelf(t *testing.T) {
	mc := MessageContext{
		ChannelName: "dev",
		Messages: []MessageInfo{
			{User: "U1", UserName: "me", Text: "my message", Ts: "100.0", IsSelf: true},
		},
	}

	text := formatConversation(mc)
	if !contains(text, "[you]") {
		t.Error("expected [you] tag for self messages")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
