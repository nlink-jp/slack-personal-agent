// Package keychain provides an abstraction over OS-native secret storage.
package keychain

import "github.com/zalando/go-keyring"

const serviceName = "slack-personal-agent"

// Store defines the interface for OS secret storage.
// Implementations include OSStore (production) and MockStore (tests).
type Store interface {
	Get(key string) (string, error)
	Set(key, secret string) error
	Delete(key string) error
}

// OSStore uses the native OS keychain (macOS Keychain, Linux libsecret,
// Windows Credential Manager).
type OSStore struct{}

// Get retrieves the secret for the given key from the OS keychain.
func (s *OSStore) Get(key string) (string, error) {
	return keyring.Get(serviceName, key)
}

// Set stores the secret for the given key in the OS keychain.
func (s *OSStore) Set(key, secret string) error {
	return keyring.Set(serviceName, key, secret)
}

// Delete removes the secret for the given key from the OS keychain.
func (s *OSStore) Delete(key string) error {
	return keyring.Delete(serviceName, key)
}

// WorkspaceTokenKey returns the keychain key for a workspace token.
func WorkspaceTokenKey(workspace string) string {
	return "workspace:" + workspace
}

// LLMAPIKeyKey returns the keychain key for an LLM backend API key.
func LLMAPIKeyKey(backend string) string {
	return "llm:" + backend
}
