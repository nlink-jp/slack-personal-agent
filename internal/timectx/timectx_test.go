package timectx

import (
	"strings"
	"testing"
	"time"
)

func TestForTime(t *testing.T) {
	// Wednesday, 2026-04-22 14:30:00 JST
	loc, _ := time.LoadLocation("Asia/Tokyo")
	tm := time.Date(2026, 4, 22, 14, 30, 0, 0, loc)

	got := ForTime(tm)

	checks := []struct {
		label    string
		contains string
	}{
		{"date", "2026-04-22 14:30:00"},
		{"timezone", "Asia/Tokyo"},
		{"weekday", "Wednesday"},
		{"iso week", "2026-W17"},
		{"days in month", "Days in this month: 30"},
	}

	for _, c := range checks {
		if !strings.Contains(got, c.contains) {
			t.Errorf("expected %s to contain %q, got:\n%s", c.label, c.contains, got)
		}
	}
}

func TestForTimeMonday(t *testing.T) {
	// Monday, 2026-01-05
	tm := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	got := ForTime(tm)

	if !strings.Contains(got, "Monday") {
		t.Errorf("expected Monday, got:\n%s", got)
	}
	if !strings.Contains(got, "2026-W02") {
		t.Errorf("expected ISO week 2026-W02, got:\n%s", got)
	}
}

func TestForTimeFebruary(t *testing.T) {
	// February 2026 has 28 days
	tm := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	got := ForTime(tm)

	if !strings.Contains(got, "Days in this month: 28") {
		t.Errorf("expected 28 days in Feb 2026, got:\n%s", got)
	}
}

func TestForTimeLeapYear(t *testing.T) {
	// February 2028 is a leap year (29 days)
	tm := time.Date(2028, 2, 15, 12, 0, 0, 0, time.UTC)
	got := ForTime(tm)

	if !strings.Contains(got, "Days in this month: 29") {
		t.Errorf("expected 29 days in Feb 2028, got:\n%s", got)
	}
}

func TestNowContainsAllFields(t *testing.T) {
	got := Now()

	required := []string{
		"Current date and time:",
		"Timezone:",
		"Day of week:",
		"ISO week:",
		"Days in this month:",
	}

	for _, r := range required {
		if !strings.Contains(got, r) {
			t.Errorf("Now() missing field %q, got:\n%s", r, got)
		}
	}
}
