package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/shiron-dev/ai-baton/internal/config"
	"github.com/shiron-dev/ai-baton/internal/review"
)

var rootCmd = &cobra.Command{
	Use:   "review-harness",
	Short: "AI Review Memory Harness - accumulates review knowledge across PRs",
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	rootCmd.AddCommand(reviewCmd(), syncCmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func reviewCmd() *cobra.Command {
	var (
		repo        string
		prNumber    int
		configPath  string
		agentName   string
		dryRun      bool
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

			opts := review.PipelineOptions{
				Cfg:          cfg,
				Repo:         repo,
				GithubToken:  requireEnv("GITHUB_TOKEN"),
				OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
				AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
				DryRun:       dryRun,
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print findings without posting to GitHub")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("pr")
	return cmd
}

func syncCmd() *cobra.Command {
	var (
		repo       string
		prNumber   int
		configPath string
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
				Cfg:          cfg,
				Repo:         repo,
				GithubToken:  requireEnv("GITHUB_TOKEN"),
				OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
				AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
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
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("pr")
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
