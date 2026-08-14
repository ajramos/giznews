// Package fetch turns raw source items into persisted, de-duplicated articles.
package fetch

import (
	"hash/fnv"
	"math/bits"
	"strings"
)

// SimHash computes a 64-bit locality-sensitive fingerprint for text. Near-duplicate
// documents yield fingerprints with small Hamming distance. It is used for
// cross-source duplicate detection (the same story appearing on multiple feeds).
func SimHash(text string) uint64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return 0
	}

	const bits64 = 64
	var sums [bits64]int
	for _, tok := range tokens {
		h := fnv.New64a()
		h.Write([]byte(tok))
		hv := h.Sum64()
		for i := 0; i < bits64; i++ {
			if hv&(1<<uint(i)) != 0 {
				sums[i]++
			} else {
				sums[i]--
			}
		}
	}

	var out uint64
	for i := 0; i < bits64; i++ {
		if sums[i] > 0 {
			out |= 1 << uint(i)
		}
	}
	return out
}

// HammingDistance counts differing bits between two simhashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// tokenize lowercases and splits text into alpha-numeric tokens.
func tokenize(text string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(f) >= 3 { // skip tiny filler tokens
			out = append(out, f)
		}
	}
	return out
}
