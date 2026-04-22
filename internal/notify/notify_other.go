//go:build !darwin

package notify

import (
	"context"
	"log"
)

// Send is a no-op on non-macOS platforms.
func Send(_ context.Context, title, body string) error {
	log.Printf("notification (no native support): %s — %s", title, body)
	return nil
}

// SendWithSubtitle is a no-op on non-macOS platforms.
func SendWithSubtitle(_ context.Context, title, subtitle, body string) error {
	log.Printf("notification (no native support): %s — %s — %s", title, subtitle, body)
	return nil
}
