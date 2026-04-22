//go:build darwin

// Package notify provides macOS native notifications via osascript.
package notify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Send sends a macOS notification using osascript.
// title is the notification title, body is the notification body.
func Send(ctx context.Context, title, body string) error {
	script := fmt.Sprintf(
		`display notification %s with title %s`,
		appleScriptString(body),
		appleScriptString(title),
	)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	return cmd.Run()
}

// SendWithSubtitle sends a macOS notification with a subtitle.
func SendWithSubtitle(ctx context.Context, title, subtitle, body string) error {
	script := fmt.Sprintf(
		`display notification %s with title %s subtitle %s`,
		appleScriptString(body),
		appleScriptString(title),
		appleScriptString(subtitle),
	)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	return cmd.Run()
}

// appleScriptString escapes a string for AppleScript.
func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
