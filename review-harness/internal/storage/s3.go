package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config holds the configuration for the S3 storage backend.
type S3Config struct {
	Bucket string
	Key    string
	Region string
}

// S3Backend downloads and uploads the SQLite database to/from S3.
// AWS credentials are resolved via the standard credential chain
// (env vars, ~/.aws/credentials, instance profile, OIDC, etc.).
type S3Backend struct {
	client *s3.Client
	bucket string
	key    string
}

// NewS3Backend creates an S3Backend, loading AWS config from the environment.
func NewS3Backend(ctx context.Context, cfg S3Config) (*S3Backend, error) {
	opts := []func(*config.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &S3Backend{
		client: s3.NewFromConfig(awsCfg),
		bucket: cfg.Bucket,
		key:    cfg.Key,
	}, nil
}

// Fetch downloads the database from S3 to localPath.
// If the object does not exist, Fetch is silently skipped (first run).
func (b *S3Backend) Fetch(ctx context.Context, localPath string) error {
	slog.Info("storage: fetching memory DB from S3", "bucket", b.bucket, "key", b.key)

	resp, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.key),
	})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			slog.Info("storage: no existing DB in S3, starting fresh")
			return nil
		}
		return fmt.Errorf("s3 GetObject: %w", err)
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local db file: %w", err)
	}
	defer f.Close()

	if _, err := f.ReadFrom(resp.Body); err != nil {
		return fmt.Errorf("write db file: %w", err)
	}
	slog.Info("storage: DB fetched from S3")
	return nil
}

// Flush uploads the local database at localPath to S3.
func (b *S3Backend) Flush(ctx context.Context, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to flush (dry-run or no activity)
		}
		return fmt.Errorf("open local db: %w", err)
	}
	defer f.Close()

	slog.Info("storage: flushing memory DB to S3", "bucket", b.bucket, "key", b.key)
	_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.key),
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("s3 PutObject: %w", err)
	}
	slog.Info("storage: DB flushed to S3")
	return nil
}
