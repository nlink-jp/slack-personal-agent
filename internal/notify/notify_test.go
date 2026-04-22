package notify

import "testing"

func TestAppleScriptString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{`say "hi"`, `"say \"hi\""`},
		{`path\to\file`, `"path\\to\\file"`},
		{"line1\nline2", `"line1 line2"`},
		{"inject\r\ndo shell script", `"inject  do shell script"`},
		{"tab\there", `"tab here"`},
		{"", `""`},
	}
	for _, tt := range tests {
		got := appleScriptString(tt.input)
		if got != tt.want {
			t.Errorf("appleScriptString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
