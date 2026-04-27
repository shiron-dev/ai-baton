package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version int           `yaml:"version"`
	Agent   AgentConfig   `yaml:"agent"`
	Review  ReviewConfig  `yaml:"review"`
	Context ContextConfig `yaml:"context"`
	Memory  MemoryConfig  `yaml:"memory"`
	Judge   JudgeConfig   `yaml:"judge"`
	Labels  LabelConfig   `yaml:"labels"`
}

type AgentConfig struct {
	Default string                  `yaml:"default"`
	Agents  map[string]AgentDef     `yaml:"agents"`
}

type AgentDef struct {
	Command string        `yaml:"command"`
	Args    []string      `yaml:"args"`
	Timeout time.Duration `yaml:"timeout"`
}

type ReviewConfig struct {
	MaxComments   int     `yaml:"max_comments"`
	MinConfidence float64 `yaml:"min_confidence"`
	PostInline    bool    `yaml:"post_inline"`
	PostSummary   bool    `yaml:"post_summary"`
}

type ContextConfig struct {
	MaxTokens           int  `yaml:"max_tokens"`
	IncludeRelatedTests bool `yaml:"include_related_tests"`
	IncludeCallers      bool `yaml:"include_callers"`
	IncludeDocs         bool `yaml:"include_docs"`
}

type MemoryConfig struct {
	Backend   string          `yaml:"backend"`
	Path      string          `yaml:"path"`
	Retrieval RetrievalConfig `yaml:"retrieval"`
	Embedding EmbeddingConfig `yaml:"embedding"`
}

type RetrievalConfig struct {
	VectorWeight       float64 `yaml:"vector_weight"`
	FullTextWeight     float64 `yaml:"full_text_weight"`
	CodeLocationWeight float64 `yaml:"code_location_weight"`
	SymbolWeight       float64 `yaml:"symbol_weight"`
	LabelWeight        float64 `yaml:"label_weight"`
}

type EmbeddingConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type JudgeConfig struct {
	Enabled              bool   `yaml:"enabled"`
	MaxCandidates        int    `yaml:"max_candidates"`
	SuppressFalsePositive bool  `yaml:"suppress_false_positive"`
	Provider             string `yaml:"provider"`
	Model                string `yaml:"model"`
}

type LabelConfig struct {
	Lightweight bool     `yaml:"lightweight"`
	Enabled     []string `yaml:"enabled"`
}

func DefaultConfig() *Config {
	return &Config{
		Version: 1,
		Agent: AgentConfig{
			Default: "claude",
			Agents: map[string]AgentDef{
				"codex": {
					Command: "codex",
					Args:    []string{"exec"},
					Timeout: 600 * time.Second,
				},
				"claude": {
					Command: "claude",
					Args:    []string{"-p"},
					Timeout: 600 * time.Second,
				},
			},
		},
		Review: ReviewConfig{
			MaxComments:   20,
			MinConfidence: 0.65,
			PostInline:    true,
			PostSummary:   true,
		},
		Context: ContextConfig{
			MaxTokens:           50000,
			IncludeRelatedTests: true,
			IncludeCallers:      true,
			IncludeDocs:         true,
		},
		Memory: MemoryConfig{
			Backend: "sqlite",
			Path:    ".review-harness/memory.sqlite",
			Retrieval: RetrievalConfig{
				VectorWeight:       0.45,
				FullTextWeight:     0.25,
				CodeLocationWeight: 0.15,
				SymbolWeight:       0.10,
				LabelWeight:        0.05,
			},
			Embedding: EmbeddingConfig{
				Provider: "openai",
				Model:    "text-embedding-3-small",
			},
		},
		Judge: JudgeConfig{
			Enabled:              true,
			MaxCandidates:        10,
			SuppressFalsePositive: true,
			Provider:             "anthropic",
			Model:                "claude-haiku-4-5-20251001",
		},
		Labels: LabelConfig{
			Lightweight: true,
			Enabled: []string{
				"bug", "security", "performance",
				"maintainability", "test-missing", "project-convention",
			},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}
