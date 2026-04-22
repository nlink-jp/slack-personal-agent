package keychain

import "fmt"

// MockStore is an in-memory Store implementation for testing.
type MockStore struct {
	data map[string]string
}

// NewMockStore creates a new MockStore.
func NewMockStore() *MockStore {
	return &MockStore{data: make(map[string]string)}
}

// Get retrieves the secret for the given key.
func (m *MockStore) Get(key string) (string, error) {
	v, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", key)
	}
	return v, nil
}

// Set stores the secret for the given key.
func (m *MockStore) Set(key, secret string) error {
	m.data[key] = secret
	return nil
}

// Delete removes the secret for the given key.
func (m *MockStore) Delete(key string) error {
	if _, ok := m.data[key]; !ok {
		return fmt.Errorf("secret not found: %s", key)
	}
	delete(m.data, key)
	return nil
}
