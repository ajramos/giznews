package sources

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// HTMLToMarkdown converts HTML article bodies to markdown for the reader and
// the knowledge base. Failures degrade to plain text.
func HTMLToMarkdown(raw string) string {
	return htmlToMarkdown(raw)
}

// htmlToMarkdown converts HTML article bodies to markdown for the reader and
// the knowledge base. Failures degrade to plain text.
func htmlToMarkdown(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	out, err := md.ConvertString(raw)
	if err != nil {
		return stripTags(raw)
	}
	return strings.TrimSpace(out)
}

// stripTags is a last-resort fallback that removes HTML tags without a real
// parser, keeping text content readable.
func stripTags(raw string) string {
	var b strings.Builder
	depth := 0
	in := false
	for _, r := range raw {
		switch {
		case r == '<':
			in = true
			depth++
		case r == '>':
			in = false
			depth--
		case !in && depth <= 0:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
