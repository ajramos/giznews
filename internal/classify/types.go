// Package classify assigns every article a category, importance score, tags,
// entities and a summary. It runs deterministic rules first (fast, ⚡) and only
// sends the remaining articles to the LLM in batches, mirroring giztui's
// inbox-analyzer prefilter pattern.
package classify

import "github.com/ajramos/giznews/internal/db"

// Categories is the stable AI-news taxonomy used by the LLM prompt and the
// knowledge graph. Keeping it fixed makes digests and notes consistent.
var Categories = []string{
	"models",      // new models / releases / benchmarks
	"research",    // papers, techniques, open problems
	"industry",    // company news, deals, products, launches
	"funding",     // fundraising, valuations, M&A
	"regulation",  // policy, safety, governance, lawsuits
	"tools",       // developer tools, libraries, SDKs, platforms
	"open-source", // open models / weights / community
	"opinion",     // commentary, essays, analysis
	"general",     // everything else
}

// Classification is the outcome for one article.
type Classification struct {
	ArticleID  int64       `json:"id"`
	Category   string      `json:"category"`
	Importance int         `json:"importance"` // 0..3
	Tags       []string    `json:"tags"`
	Entities   []db.Entity `json:"entities"`
	Summary    string      `json:"summary"`
}

// Result summarizes a classification run.
type Result struct {
	Classified   int      `json:"classified"`
	ByRules      int      `json:"by_rules"`
	Archived     int      `json:"archived"`
	Boosted      int      `json:"boosted"`
	ByLLM        int      `json:"by_llm"`
	SkippedNoLLM int      `json:"skipped_no_llm"`
	Batches      int      `json:"batches"`
	// Pending counts articles a rules-only run left unclassified, waiting for
	// the model.
	Pending int      `json:"pending"`
	Errors  []string `json:"errors,omitempty"`
}
