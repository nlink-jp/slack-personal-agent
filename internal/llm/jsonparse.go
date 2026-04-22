package llm

import (
	"encoding/json"
	"fmt"

	"github.com/nlink-jp/nlk/jsonfix"
)

// ExtractJSON extracts and repairs JSON from an LLM response.
// Handles markdown fences, trailing text, and common JSON malformations.
func ExtractJSON(text string) (json.RawMessage, error) {
	raw, err := jsonfix.Extract(text)
	if err != nil {
		return nil, fmt.Errorf("extract JSON from LLM response: %w", err)
	}
	return json.RawMessage(raw), nil
}

// ExtractJSONTo extracts JSON from an LLM response and unmarshals into target.
func ExtractJSONTo(text string, target interface{}) error {
	raw, err := ExtractJSON(text)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
