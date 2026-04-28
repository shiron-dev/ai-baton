package storage

import "context"

// Backend persists the SQLite memory database across workflow runs.
// Fetch is called before the review starts; Flush is called after it completes.
type Backend interface {
	// Fetch downloads the remote database to localPath.
	// If no remote database exists yet (first run), Fetch is a no-op.
	Fetch(ctx context.Context, localPath string) error
	// Flush uploads the local database at localPath to the remote store.
	Flush(ctx context.Context, localPath string) error
}
