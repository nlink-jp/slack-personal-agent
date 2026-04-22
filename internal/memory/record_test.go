package memory

import (
	"testing"
	"time"
)

func TestParseSlackTs(t *testing.T) {
	tests := []struct {
		ts   string
		want time.Time
	}{
		{"1713488400.000100", time.Unix(1713488400, 100000)},
		{"1713488400.000000", time.Unix(1713488400, 0)},
		{"1713488400", time.Unix(1713488400, 0)},
		{"", time.Time{}},
	}

	for _, tt := range tests {
		got := ParseSlackTs(tt.ts)
		if !got.Equal(tt.want) {
			t.Errorf("ParseSlackTs(%q) = %v, want %v", tt.ts, got, tt.want)
		}
	}
}

func TestRecordSlackTimestamp(t *testing.T) {
	r := &Record{Ts: "1713488400.000100"}
	ts := r.SlackTimestamp()

	if ts.Unix() != 1713488400 {
		t.Errorf("expected unix 1713488400, got %d", ts.Unix())
	}
}
