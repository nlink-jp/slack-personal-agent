package llm

import "github.com/nlink-jp/nlk/guard"

// WrapUserContent wraps untrusted user content (Slack messages, RAG results)
// in nonce-tagged XML for prompt injection defense.
// A new tag is generated per call to prevent tag reuse across turns.
func WrapUserContent(content string) (wrapped string, tagName string, err error) {
	tag := guard.NewTag()
	wrapped, err = tag.Wrap(content)
	if err != nil {
		return "", "", err
	}
	return wrapped, tag.Name(), nil
}

// ExpandPrompt replaces {{DATA_TAG}} in a system prompt with the actual tag name.
func ExpandPrompt(systemPrompt string, tag guard.Tag) string {
	return tag.Expand(systemPrompt)
}

// NewGuardTag creates a new guard tag for a single LLM turn.
func NewGuardTag() guard.Tag {
	return guard.NewTag()
}
