package fetch

import "strings"

// Near-duplicate detection for headlines needs a different measure than the one
// that catches an article republished verbatim.
//
// SimHash is built for documents. On a nine-word headline it is brittle in the
// worst way: adding one word ("… today", "… rules") moves it 11-14 bits, far
// past any threshold that unrelated stories would not also cross. Two
// newsrooms writing the same story almost never write the same words, so a
// document fingerprint would have grouped almost nothing.
//
// What two reports of one event do share is their nouns. Comparing the sets of
// meaningful words — Jaccard over the title, minus the words every headline
// contains — is stable under the edits newsrooms actually make: a trailing
// clause, a dropped article, a rewritten verb.

// titleStopwords are the words a headline can gain or lose without becoming a
// different story.
var titleStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "with": true,
	"from": true, "by": true, "as": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "it": true, "its": true, "this": true, "that": true,
	"has": true, "have": true, "will": true, "after": true, "over": true,
	"into": true, "amid": true, "says": true, "say": true, "said": true,
	"new": true, "now": true, "today": true, "here": true, "you": true,
}

// TitleTokens reduces a headline to the words that decide what it is about.
func TitleTokens(title string) []string {
	var (
		out  []string
		word strings.Builder
	)
	flush := func() {
		if word.Len() == 0 {
			return
		}
		w := word.String()
		word.Reset()
		if len(w) < 2 || titleStopwords[w] {
			return
		}
		out = append(out, w)
	}
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			word.WriteRune(r)
		case r == '-' || r == '.':
			// Kept inside a word (gpt-5, 3.5) and dropped at its edges.
			if word.Len() > 0 {
				word.WriteRune(r)
			}
		default:
			flush()
		}
	}
	flush()
	for i, w := range out {
		out[i] = strings.Trim(w, "-.")
	}
	return out
}

// TitleSimilarity is the Jaccard overlap of two token sets: 1 when they say the
// same thing, 0 when they share nothing.
func TitleSimilarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, w := range a {
		set[w] = true
	}
	shared := 0
	seen := make(map[string]bool, len(b))
	for _, w := range b {
		if seen[w] {
			continue
		}
		seen[w] = true
		if set[w] {
			shared++
		}
	}
	union := len(set) + len(seen) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

const (
	// titleSimilarityMin is how much two headlines must overlap to be one
	// story, and titleSharedMin how many meaningful words they must actually
	// share.
	//
	// The ratio alone does not separate them. "Apple brings AI to the iPhone"
	// and "…to the iPad" overlap 0.60 — as much as a genuine rewrite — and
	// differ in the only word that matters, while "Anthropic raises $10B at a
	// $350B valuation" and "Anthropic closes $10B round at $350B valuation"
	// are the same story at 0.57. What tells them apart is how many meaningful
	// words they actually share: four against three. So the count does the
	// discriminating and the ratio is only a floor, there to stop two long
	// unrelated headlines from meeting the count by accident.
	titleSimilarityMin = 0.55
	titleSharedMin     = 4
)

// SameStory reports whether two headlines describe the same event.
//
// It errs towards saying no. A missed pair costs a duplicate row in the list,
// which the reader can see and ignore; a false pair hides an article behind
// another one, where nobody will ever look for it.
func SameStory(a, b []string) bool {
	shared := sharedTokens(a, b)
	if shared < titleSharedMin {
		return false
	}
	if !versionsAgree(a, b) {
		return false // GPT-5 and GPT-4.5 are not the same announcement
	}
	return TitleSimilarity(a, b) >= titleSimilarityMin
}

func sharedTokens(a, b []string) int {
	set := make(map[string]bool, len(a))
	for _, w := range a {
		set[w] = true
	}
	n := 0
	seen := make(map[string]bool, len(b))
	for _, w := range b {
		if seen[w] || !set[w] {
			continue
		}
		seen[w] = true
		n++
	}
	return n
}

// versionsAgree compares the tokens carrying a digit — model versions, chip
// names, amounts. When both headlines name some and they differ, they are
// reporting different things however alike they read.
func versionsAgree(a, b []string) bool {
	va, vb := versionTokens(a), versionTokens(b)
	if len(va) == 0 || len(vb) == 0 {
		return true // nothing to compare
	}
	for w := range va {
		if vb[w] {
			return true // they name at least one thing in common
		}
	}
	return false
}

func versionTokens(tokens []string) map[string]bool {
	out := map[string]bool{}
	for _, w := range tokens {
		for _, r := range w {
			if r >= '0' && r <= '9' {
				out[w] = true
				break
			}
		}
	}
	return out
}
