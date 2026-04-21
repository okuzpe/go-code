package memory

import (
	"math"
	"strings"
)

// BM25 ranking parameters. k1 controls term-frequency saturation (higher = more weight to repeated
// terms); b controls length normalization (1.0 = full normalization, 0 = none).
// These are the canonical Okapi BM25 defaults used by Elasticsearch and most IR systems.
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// bm25Corpus holds the pre-computed statistics needed to score documents with BM25.
type bm25Corpus struct {
	docs    [][]string     // tokenized documents, one slice per doc (duplicates preserved for TF)
	docFreq map[string]int // number of documents that contain each term (for IDF)
	avgdl   float64        // average document length in tokens
}

// newBM25Corpus builds corpus statistics from a set of pre-tokenized documents.
func newBM25Corpus(docs [][]string) bm25Corpus {
	docFreq := make(map[string]int)
	totalLen := 0
	for _, doc := range docs {
		totalLen += len(doc)
		seen := make(map[string]struct{}, len(doc))
		for _, term := range doc {
			if _, already := seen[term]; !already {
				docFreq[term]++
				seen[term] = struct{}{}
			}
		}
	}
	avgdl := 1.0 // guard against zero-length corpus
	if len(docs) > 0 {
		avgdl = float64(totalLen) / float64(len(docs))
		if avgdl == 0 {
			avgdl = 1
		}
	}
	return bm25Corpus{docs: docs, docFreq: docFreq, avgdl: avgdl}
}

// score returns the BM25 score for the document at docIdx against the provided query terms.
// Returns 0 if the document contains none of the query terms.
func (c bm25Corpus) score(docIdx int, queryTerms []string) float64 {
	if len(queryTerms) == 0 || docIdx < 0 || docIdx >= len(c.docs) {
		return 0
	}
	doc := c.docs[docIdx]
	docLen := float64(len(doc))
	N := float64(len(c.docs))

	// Build per-document term-frequency map.
	tf := make(map[string]int, len(doc))
	for _, term := range doc {
		tf[term]++
	}

	var total float64
	for _, term := range queryTerms {
		termFreq := float64(tf[term])
		if termFreq == 0 {
			continue
		}
		// Robertson-Sparck Jones IDF (lower-bounded at 0 by the +1 outside the log).
		docCount := float64(c.docFreq[term])
		idf := math.Log((N-docCount+0.5)/(docCount+0.5) + 1)

		// TF with saturation (k1) and length normalization (b).
		denom := termFreq + bm25K1*(1-bm25B+bm25B*docLen/c.avgdl)
		tfNorm := termFreq * (bm25K1 + 1) / denom

		total += idf * tfNorm
	}
	return total
}

// tokenizeTerms lowercases and splits text into tokens (3+ chars, stop words removed),
// preserving duplicate occurrences so BM25 can compute term frequency correctly.
func tokenizeTerms(s string) []string {
	var tokens []string
	for _, word := range strings.Fields(strings.ToLower(s)) {
		word = strings.Trim(word, ".,!?;:\"'`()[]{}<>")
		if len(word) >= 3 && !isStopWord(word) {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// uniqueTerms returns the deduplicated query terms for BM25 scoring.
// Deduplication avoids double-counting the same IDF contribution for repeated query words.
func uniqueTerms(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	unique := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			unique = append(unique, t)
		}
	}
	return unique
}
