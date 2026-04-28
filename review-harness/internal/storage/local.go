package storage

import "context"

// LocalBackend is a no-op backend that relies on the local filesystem.
// The SQLite file is read/written directly at the configured path.
type LocalBackend struct{}

func (LocalBackend) Fetch(_ context.Context, _ string) error { return nil }
func (LocalBackend) Flush(_ context.Context, _ string) error { return nil }
