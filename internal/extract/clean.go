package extract

import "strings"

// CleanMarkdown post-processes extracted content so it renders well in the
// reader:
//
//   - Fenced code blocks that actually contain raw HTML (a common artifact of
//     html→markdown conversion of complex pages) are unwrapped so the HTML can
//     be rendered properly by the frontend (rehype-raw) instead of showing as
//     a wall of literal markup.
//   - Runs of 3+ blank lines collapse to a single blank line.
func CleanMarkdown(md string) string {
	if md == "" {
		return md
	}
	lines := strings.Split(md, "\n")
	var out []string
	var fence []string
	fenceLang := ""
	inFence := false

	flushFence := func() {
		if len(fence) == 0 {
			return
		}
		joined := strings.Join(fence, "\n")
		trimmed := strings.TrimSpace(joined)
		if strings.HasPrefix(trimmed, "<") {
			// raw HTML: emit without fences so rehype-raw can render it
			out = append(out, joined)
		} else {
			open := "```"
			if fenceLang != "" {
				open += fenceLang
			}
			out = append(out, open)
			out = append(out, fence...)
			out = append(out, "```")
		}
		fence = nil
		fenceLang = ""
	}

	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			if !inFence {
				inFence = true
				fence = nil
				fenceLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "```"))
			} else {
				inFence = false
				flushFence()
			}
			continue
		}
		if inFence {
			fence = append(fence, l)
		} else {
			out = append(out, l)
		}
	}
	if inFence {
		flushFence()
	}

	// collapse 3+ blank lines to one
	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n\n", "\n\n\n")
	}
	return strings.TrimSpace(joined)
}
