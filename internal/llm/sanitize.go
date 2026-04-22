package llm

import "github.com/nlink-jp/nlk/strip"

// SanitizeResponse removes thinking/reasoning tags from LLM output.
// These tags are emitted by local models (DeepSeek R1, Qwen, Gemma 4, etc.)
// and should be stripped before displaying to the user or further processing.
func SanitizeResponse(text string) string {
	return strip.ThinkTags(text)
}
