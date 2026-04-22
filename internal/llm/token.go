package llm

import "unicode"

// EstimateTokenCount estimates the token count using the dual-method approach.
// Returns max(word-based, char-based) to avoid underestimation,
// especially for JSON content with heavy punctuation.
func EstimateTokenCount(text string) int {
	wordBased := estimateWordBased(text)
	charBased := len(text) / 4
	if wordBased > charBased {
		return wordBased
	}
	return charBased
}

func estimateWordBased(text string) int {
	var cjkChars, asciiWords, punctuation int
	inWord := false

	for _, r := range text {
		if isCJK(r) {
			cjkChars++
			inWord = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				asciiWords++
				inWord = true
			}
		} else {
			inWord = false
			if unicode.IsPunct(r) || isJSONPunct(r) {
				punctuation++
			}
		}
	}

	return int(float64(cjkChars)*2.0 + float64(asciiWords)*1.3 + float64(punctuation))
}

func isCJK(r rune) bool {
	return unicode.In(r,
		unicode.Han,
		unicode.Hiragana,
		unicode.Katakana,
		unicode.Hangul,
	)
}

func isJSONPunct(r rune) bool {
	return r == '{' || r == '}' || r == '[' || r == ']' || r == '"' || r == ':'
}
