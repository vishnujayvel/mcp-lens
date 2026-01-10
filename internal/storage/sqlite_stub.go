//go:build !sqlite

package storage

import (
	"fmt"
)

// SQLiteStore is a stub when SQLite is not available.
type SQLiteStore struct {
	*MockStore
}

// NewSQLiteStore creates a mock store when SQLite is not available.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	fmt.Printf("Warning: SQLite not available, using in-memory storage\n")
	return &SQLiteStore{MockStore: NewMockStore()}, nil
}
