package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	gcs "cloud.google.com/go/storage"
)

// CloudStorageConfig holds the configuration for the Google Cloud Storage backend.
type CloudStorageConfig struct {
	Bucket string
	Object string
}

// CloudStorageBackend downloads and uploads the SQLite database to/from GCS.
// Google credentials are resolved via Application Default Credentials
// (Workload Identity Federation, GOOGLE_APPLICATION_CREDENTIALS, gcloud, etc.).
type CloudStorageBackend struct {
	client *gcs.Client
	bucket string
	object string
}

// NewCloudStorageBackend creates a CloudStorageBackend using ADC.
func NewCloudStorageBackend(ctx context.Context, cfg CloudStorageConfig) (*CloudStorageBackend, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud storage client: %w", err)
	}
	return &CloudStorageBackend{
		client: client,
		bucket: cfg.Bucket,
		object: cfg.Object,
	}, nil
}

// Fetch downloads the database from GCS to localPath.
// If the object does not exist, Fetch is silently skipped (first run).
func (b *CloudStorageBackend) Fetch(ctx context.Context, localPath string) error {
	slog.Info("storage: fetching memory DB from Cloud Storage", "bucket", b.bucket, "object", b.object)

	r, err := b.client.Bucket(b.bucket).Object(b.object).NewReader(ctx)
	if err != nil {
		if errors.Is(err, gcs.ErrObjectNotExist) {
			slog.Info("storage: no existing DB in Cloud Storage, starting fresh")
			return nil
		}
		return fmt.Errorf("cloudstorage NewReader: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local db file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write db file: %w", err)
	}
	slog.Info("storage: DB fetched from Cloud Storage")
	return nil
}

// Flush uploads the local database at localPath to GCS.
func (b *CloudStorageBackend) Flush(ctx context.Context, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to flush (dry-run or no activity)
		}
		return fmt.Errorf("open local db: %w", err)
	}
	defer f.Close()

	slog.Info("storage: flushing memory DB to Cloud Storage", "bucket", b.bucket, "object", b.object)
	w := b.client.Bucket(b.bucket).Object(b.object).NewWriter(ctx)
	if _, err := io.Copy(w, f); err != nil {
		_ = w.Close()
		return fmt.Errorf("cloudstorage write object: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("cloudstorage close writer: %w", err)
	}
	slog.Info("storage: DB flushed to Cloud Storage")
	return nil
}
