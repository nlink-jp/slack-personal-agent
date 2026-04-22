package agent

import (
	"github.com/nlink-jp/nlk/jsonfix"
	"encoding/json"
	"fmt"
)

// evalResponse is the expected JSON structure from the evaluation LLM call.
type evalResponse struct {
	Verdict  Verdict `json:"verdict"`
	Summary  string  `json:"summary"`
	Reason   string  `json:"reason"`
	ThreadTs string  `json:"thread_ts"`
}

// parseEvalResponse extracts the evaluation JSON from the LLM response.
func parseEvalResponse(text string) (*evalResponse, error) {
	raw, err := jsonfix.Extract(text)
	if err != nil {
		return nil, fmt.Errorf("extract JSON: %w", err)
	}

	var resp evalResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	// Validate verdict
	switch resp.Verdict {
	case VerdictIgnore, VerdictNote, VerdictRespond, VerdictReview:
		// valid
	default:
		return nil, fmt.Errorf("unknown verdict: %q", resp.Verdict)
	}

	return &resp, nil
}
