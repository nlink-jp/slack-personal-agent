// Package timectx provides full calendar context for LLM system prompts.
// Includes date, time, timezone, day of week, and ISO week number to enable
// accurate resolution of relative time expressions like "last Friday",
// "this week", "yesterday".
package timectx

import (
	"fmt"
	"time"
)

// Now returns the full calendar context string for the current moment.
func Now() string {
	return ForTime(time.Now())
}

// ForTime returns the full calendar context string for the given time.
func ForTime(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf(
		"Current date and time: %s\n"+
			"Timezone: %s\n"+
			"Day of week: %s\n"+
			"ISO week: %d-W%02d\n"+
			"Days in this month: %d",
		t.Format("2006-01-02 15:04:05"),
		t.Location().String(),
		t.Weekday().String(),
		year, week,
		daysInMonth(t),
	)
}

// daysInMonth returns the number of days in the month of the given time.
func daysInMonth(t time.Time) int {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, t.Location()).Day()
}
