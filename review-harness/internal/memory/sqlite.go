package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/shiron-dev/ai-baton/internal/schema"
	_ "modernc.org/sqlite"
)

const ddl = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS review_comments (
	id                  TEXT PRIMARY KEY,
	repo                TEXT NOT NULL,
	pr_number           INTEGER NOT NULL,
	agent               TEXT NOT NULL,
	file_path           TEXT NOT NULL,
	symbol              TEXT NOT NULL DEFAULT '',
	raw_comment         TEXT NOT NULL,
	canonical_claim     TEXT NOT NULL DEFAULT '',
	code_context_summary TEXT NOT NULL DEFAULT '',
	outcome             TEXT NOT NULL DEFAULT 'pending',
	outcome_summary     TEXT NOT NULL DEFAULT '',
	created_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS human_replies (
	id                TEXT PRIMARY KEY,
	review_comment_id TEXT NOT NULL REFERENCES review_comments(id),
	author            TEXT NOT NULL,
	body              TEXT NOT NULL,
	created_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comment_embeddings (
	review_comment_id TEXT PRIMARY KEY REFERENCES review_comments(id),
	embedding         BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS judge_results (
	id                     TEXT PRIMARY KEY,
	finding_id             TEXT NOT NULL,
	memory_id              TEXT NOT NULL REFERENCES review_comments(id),
	same_underlying_issue  INTEGER NOT NULL DEFAULT 0,
	applies_to_current_diff INTEGER NOT NULL DEFAULT 0,
	prior_outcome          TEXT NOT NULL DEFAULT '',
	recommended_action     TEXT NOT NULL DEFAULT '',
	reason                 TEXT NOT NULL DEFAULT '',
	confidence             REAL NOT NULL DEFAULT 0,
	created_at             TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS comments_fts USING fts5(
	id UNINDEXED,
	canonical_claim,
	raw_comment,
	file_path,
	symbol,
	content=review_comments,
	content_rowid=rowid
);

CREATE TRIGGER IF NOT EXISTS review_comments_ai
AFTER INSERT ON review_comments BEGIN
	INSERT INTO comments_fts(rowid, id, canonical_claim, raw_comment, file_path, symbol)
	VALUES (new.rowid, new.id, new.canonical_claim, new.raw_comment, new.file_path, new.symbol);
END;

CREATE TRIGGER IF NOT EXISTS review_comments_au
AFTER UPDATE ON review_comments BEGIN
	INSERT INTO comments_fts(comments_fts, rowid, id, canonical_claim, raw_comment, file_path, symbol)
	VALUES ('delete', old.rowid, old.id, old.canonical_claim, old.raw_comment, old.file_path, old.symbol);
	INSERT INTO comments_fts(rowid, id, canonical_claim, raw_comment, file_path, symbol)
	VALUES (new.rowid, new.id, new.canonical_claim, new.raw_comment, new.file_path, new.symbol);
END;
`

// SQLiteStore is a Store backed by SQLite with FTS5.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) the SQLite database at the given path.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Save(ctx context.Context, m *schema.ReviewMemory) (string, error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO review_comments
			(id, repo, pr_number, agent, file_path, symbol, raw_comment,
			 canonical_claim, code_context_summary, outcome, outcome_summary, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Repo, m.PRNumber, m.Agent, m.FilePath, m.Symbol,
		m.RawComment, m.CanonicalClaim, m.CodeContextSummary,
		string(m.Outcome), m.OutcomeSummary, m.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("save memory: %w", err)
	}
	for _, r := range m.HumanReplies {
		if err := s.AddHumanReply(ctx, m.ID, r); err != nil {
			return "", err
		}
	}
	return m.ID, nil
}

func (s *SQLiteStore) UpdateOutcome(ctx context.Context, id string, outcome schema.Outcome, summary string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE review_comments SET outcome=?, outcome_summary=? WHERE id=?`,
		string(outcome), summary, id,
	)
	return err
}

func (s *SQLiteStore) AddHumanReply(ctx context.Context, memoryID string, reply schema.HumanReply) error {
	if reply.ID == "" {
		reply.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO human_replies (id, review_comment_id, author, body, created_at) VALUES (?,?,?,?,?)`,
		reply.ID, memoryID, reply.Author, reply.Body,
		reply.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) GetByID(ctx context.Context, id string) (*schema.ReviewMemory, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, repo, pr_number, agent, file_path, symbol, raw_comment,
		       canonical_claim, code_context_summary, outcome, outcome_summary, created_at
		FROM review_comments WHERE id=?`, id)
	m, err := scanMemory(row)
	if err != nil {
		return nil, err
	}
	replies, err := s.loadReplies(ctx, id)
	if err != nil {
		return nil, err
	}
	m.HumanReplies = replies
	return m, nil
}

func (s *SQLiteStore) ListByRepo(ctx context.Context, repo string, limit int) ([]*schema.ReviewMemory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo, pr_number, agent, file_path, symbol, raw_comment,
		       canonical_claim, code_context_summary, outcome, outcome_summary, created_at
		FROM review_comments WHERE repo=? ORDER BY created_at DESC LIMIT ?`, repo, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*schema.ReviewMemory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) SaveEmbedding(ctx context.Context, memoryID string, vector []float32) error {
	blob, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO comment_embeddings (review_comment_id, embedding) VALUES (?,?)`,
		memoryID, blob,
	)
	return err
}

func (s *SQLiteStore) SearchFTS(ctx context.Context, query string, limit int) ([]FTSResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, bm25(comments_fts) AS score
		FROM comments_fts
		WHERE comments_fts MATCH ?
		ORDER BY score
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()
	var results []FTSResult
	for rows.Next() {
		var r FTSResult
		var rawScore float64
		if err := rows.Scan(&r.MemoryID, &rawScore); err != nil {
			return nil, err
		}
		// bm25 returns negative values; negate to get positive score
		r.Score = -rawScore
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchVector loads all embeddings and computes cosine similarity in Go.
// For MVP this is acceptable; scale to a vector DB later.
func (s *SQLiteStore) SearchVector(ctx context.Context, query []float32, limit int) ([]VectorResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT review_comment_id, embedding FROM comment_embeddings`)
	if err != nil {
		return nil, fmt.Errorf("load embeddings: %w", err)
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		var vec []float32
		if err := json.Unmarshal(blob, &vec); err != nil {
			continue
		}
		sim := cosineSimilarity(query, vec)
		candidates = append(candidates, candidate{id, sim})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// partial sort: keep top-k
	topK := topKCandidates(candidates, limit)
	results := make([]VectorResult, len(topK))
	for i, c := range topK {
		results[i] = VectorResult{MemoryID: c.id, Similarity: c.sim}
	}
	return results, nil
}

func (s *SQLiteStore) SaveJudgeResult(ctx context.Context, r *schema.JudgeResult) error {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO judge_results
			(id, finding_id, memory_id, same_underlying_issue, applies_to_current_diff,
			 prior_outcome, recommended_action, reason, confidence, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, r.FindingID, r.MemoryID,
		boolInt(r.SameUnderlyingIssue), boolInt(r.AppliesToCurrentDiff),
		string(r.PriorOutcome), string(r.RecommendedAction),
		r.Reason, r.Confidence, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// scanRow abstracts sql.Row and sql.Rows for scanMemory.
type scanRow interface {
	Scan(dest ...any) error
}

func scanMemory(row scanRow) (*schema.ReviewMemory, error) {
	var m schema.ReviewMemory
	var outcome, createdAt string
	if err := row.Scan(
		&m.ID, &m.Repo, &m.PRNumber, &m.Agent, &m.FilePath, &m.Symbol,
		&m.RawComment, &m.CanonicalClaim, &m.CodeContextSummary,
		&outcome, &m.OutcomeSummary, &createdAt,
	); err != nil {
		return nil, err
	}
	m.Outcome = schema.Outcome(outcome)
	t, err := time.Parse(time.RFC3339, createdAt)
	if err == nil {
		m.CreatedAt = t
	}
	return &m, nil
}

func (s *SQLiteStore) loadReplies(ctx context.Context, memoryID string) ([]schema.HumanReply, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, author, body, created_at FROM human_replies WHERE review_comment_id=? ORDER BY created_at`,
		memoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var replies []schema.HumanReply
	for rows.Next() {
		var r schema.HumanReply
		var createdAt string
		if err := rows.Scan(&r.ID, &r.Author, &r.Body, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		replies = append(replies, r)
	}
	return replies, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
