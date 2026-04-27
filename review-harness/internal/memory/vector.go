package memory

import (
	"math"
	"sort"
)

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

type candidate struct {
	id  string
	sim float64
}

func topKCandidates(candidates []candidate, k int) []candidate {
	if k <= 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].sim > candidates[j].sim
	})
	if k > len(candidates) {
		k = len(candidates)
	}
	return candidates[:k]
}
