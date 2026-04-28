package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/shiron-dev/ai-baton/internal/config"
	"github.com/shiron-dev/ai-baton/internal/review"
	"github.com/spf13/cobra"
)

var (
	logFilePath   string
	logFileHandle *os.File
)

var rootCmd = &cobra.Command{
	Use:   "review-harness",
	Short: "AI Review Memory Harness - accumulates review knowledge across PRs",
}

func main() {
	rootCmd.PersistentFlags().StringVar(&logFilePath, "log-file", "", "Path to write verbose logs (Debug level) in addition to stderr (optional)")
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return setupLogger()
	}

	rootCmd.AddCommand(reviewCmd(), syncCmd(), embedCmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if logFileHandle != nil {
		logFileHandle.Close()
	}
}

// multiHandler fans out slog records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

func setupLogger() error {
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	if logFilePath == "" {
		slog.SetDefault(slog.New(stderrHandler))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	logFileHandle = f
	// File handler captures Debug and above; stderr stays at Info.
	fileHandler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(&multiHandler{handlers: []slog.Handler{stderrHandler, fileHandler}}))
	return nil
}

func reviewCmd() *cobra.Command {
	var (
		repo       string
		prNumber   int
		configPath string
		agentName  string
		agentModel string
		dryRun     bool
		// storage flags
		storageBackend     string
		s3Bucket           string
		s3Key              string
		s3Region           string
		cloudStorageBucket string
		cloudStorageObject string
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run AI review on a GitHub PR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if agentName != "" {
				cfg.Agent.Default = agentName
			}
			if agentModel != "" {
				agentDef := cfg.Agent.Agents[cfg.Agent.Default]
				agentDef.Model = agentModel
				cfg.Agent.Agents[cfg.Agent.Default] = agentDef
			}

			opts := review.PipelineOptions{
				Cfg:                cfg,
				Repo:               repo,
				GithubToken:        requireEnv("GITHUB_TOKEN"),
				OpenAIKey:          os.Getenv("OPENAI_API_KEY"),
				AnthropicKey:       os.Getenv("ANTHROPIC_API_KEY"),
				DryRun:             dryRun,
				StorageBackend:     storageBackend,
				S3Bucket:           s3Bucket,
				S3Key:              s3Key,
				S3Region:           s3Region,
				CloudStorageBucket: cloudStorageBucket,
				CloudStorageObject: cloudStorageObject,
			}
			pipeline, err := review.NewPipeline(ctx, opts)
			if err != nil {
				return fmt.Errorf("init pipeline: %w", err)
			}
			defer pipeline.Close()

			result, err := pipeline.Run(ctx, prNumber)
			if err != nil {
				return fmt.Errorf("run review: %w", err)
			}

			if dryRun {
				fmt.Printf("[dry-run] PR #%d: would post %d comment(s), suppressed %d\n",
					result.PRNumber, result.Posted, result.Suppressed)
			} else {
				fmt.Printf("PR #%d: posted %d comment(s), suppressed %d\n",
					result.PRNumber, result.Posted, result.Suppressed)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo slug owner/repo (required)")
	cmd.Flags().IntVar(&prNumber, "pr", 0, "Pull request number (required)")
	cmd.Flags().StringVar(&configPath, "config", ".review-harness.yaml", "Config file path")
	cmd.Flags().StringVar(&agentName, "agent", "", "Agent name (overrides config default)")
	cmd.Flags().StringVar(&agentModel, "agent-model", "", "Agent model (overrides config agent model)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print findings without posting to GitHub")
	cmd.Flags().StringVar(&storageBackend, "storage", "", "Storage backend: local or s3 (overrides config)")
	cmd.Flags().StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket for memory storage")
	cmd.Flags().StringVar(&s3Key, "s3-key", "", "S3 object key for memory DB")
	cmd.Flags().StringVar(&s3Region, "s3-region", "", "AWS region")
	cmd.Flags().StringVar(&cloudStorageBucket, "cloudstorage-bucket", "", "Google Cloud Storage bucket for memory storage")
	cmd.Flags().StringVar(&cloudStorageObject, "cloudstorage-object", "", "Google Cloud Storage object name for memory DB")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("pr")
	return cmd
}

func syncCmd() *cobra.Command {
	var (
		repo               string
		prNumber           int
		configPath         string
		storageBackend     string
		s3Bucket           string
		s3Key              string
		s3Region           string
		cloudStorageBucket string
		cloudStorageObject string
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync human replies and update outcomes for a PR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			opts := review.PipelineOptions{
				Cfg:                cfg,
				Repo:               repo,
				GithubToken:        requireEnv("GITHUB_TOKEN"),
				OpenAIKey:          os.Getenv("OPENAI_API_KEY"),
				AnthropicKey:       os.Getenv("ANTHROPIC_API_KEY"),
				StorageBackend:     storageBackend,
				S3Bucket:           s3Bucket,
				S3Key:              s3Key,
				S3Region:           s3Region,
				CloudStorageBucket: cloudStorageBucket,
				CloudStorageObject: cloudStorageObject,
			}
			pipeline, err := review.NewPipeline(ctx, opts)
			if err != nil {
				return fmt.Errorf("init pipeline: %w", err)
			}
			defer pipeline.Close()

			if err := pipeline.Sync(ctx, prNumber); err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			fmt.Printf("PR #%d: sync complete\n", prNumber)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo slug owner/repo (required)")
	cmd.Flags().IntVar(&prNumber, "pr", 0, "Pull request number (required)")
	cmd.Flags().StringVar(&configPath, "config", ".review-harness.yaml", "Config file path")
	cmd.Flags().StringVar(&storageBackend, "storage", "", "Storage backend: local or s3 (overrides config)")
	cmd.Flags().StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket for memory storage")
	cmd.Flags().StringVar(&s3Key, "s3-key", "", "S3 object key for memory DB")
	cmd.Flags().StringVar(&s3Region, "s3-region", "", "AWS region")
	cmd.Flags().StringVar(&cloudStorageBucket, "cloudstorage-bucket", "", "Google Cloud Storage bucket for memory storage")
	cmd.Flags().StringVar(&cloudStorageObject, "cloudstorage-object", "", "Google Cloud Storage object name for memory DB")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("pr")
	return cmd
}

func embedCmd() *cobra.Command {
	var (
		repo               string
		configPath         string
		storageBackend     string
		s3Bucket           string
		s3Key              string
		s3Region           string
		cloudStorageBucket string
		cloudStorageObject string
	)

	cmd := &cobra.Command{
		Use:   "embed",
		Short: "Generate embeddings for memory entries that lack one",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			opts := review.PipelineOptions{
				Cfg:                cfg,
				Repo:               repo,
				GithubToken:        os.Getenv("GITHUB_TOKEN"),
				OpenAIKey:          requireEnv("OPENAI_API_KEY"),
				AnthropicKey:       os.Getenv("ANTHROPIC_API_KEY"),
				StorageBackend:     storageBackend,
				S3Bucket:           s3Bucket,
				S3Key:              s3Key,
				S3Region:           s3Region,
				CloudStorageBucket: cloudStorageBucket,
				CloudStorageObject: cloudStorageObject,
			}
			pipeline, err := review.NewPipeline(ctx, opts)
			if err != nil {
				return fmt.Errorf("init pipeline: %w", err)
			}
			defer pipeline.Close()

			n, err := pipeline.EmbedAll(ctx)
			if err != nil {
				return fmt.Errorf("embed: %w", err)
			}
			fmt.Printf("embedded %d memory entries\n", n)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo slug owner/repo")
	cmd.Flags().StringVar(&configPath, "config", ".review-harness.yaml", "Config file path")
	cmd.Flags().StringVar(&storageBackend, "storage", "", "Storage backend: local or s3 (overrides config)")
	cmd.Flags().StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket for memory storage")
	cmd.Flags().StringVar(&s3Key, "s3-key", "", "S3 object key for memory DB")
	cmd.Flags().StringVar(&s3Region, "s3-region", "", "AWS region")
	cmd.Flags().StringVar(&cloudStorageBucket, "cloudstorage-bucket", "", "Google Cloud Storage bucket for memory storage")
	cmd.Flags().StringVar(&cloudStorageObject, "cloudstorage-object", "", "Google Cloud Storage object name for memory DB")
	return cmd
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: environment variable %s is required\n", key)
		os.Exit(1)
	}
	return v
}
