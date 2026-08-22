package digest

import (
	"fmt"
	"html"
	"strings"
)

// A digest that only exists inside a desktop app is one that mostly goes
// unread: digests are read on a phone, over coffee, away from the machine that
// produced them. So it has to come out as a file.
//
// Both renderers are pure functions of the digest. Nothing here reads the
// clock, because a digest exported twice must be the same bytes twice —
// otherwise you cannot tell a re-export from a change.

// Markdown renders a digest for a notes app, a commit, or a pipe.
func Markdown(d *Digest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AI digest · %s\n\n", d.Date)
	if d.Overview != "" {
		b.WriteString(d.Overview + "\n\n")
	}
	if len(d.Watchlist) > 0 {
		b.WriteString("## Watchlist\n\n")
		for _, hit := range d.Watchlist {
			fmt.Fprintf(&b, "- [%s](%s) — *%s*\n", hit.Article.Title, hit.Article.URL, hit.Rule)
		}
		b.WriteString("\n")
	}
	for _, th := range d.Themes {
		fmt.Fprintf(&b, "## %s\n\n", th.Name)
		if th.Summary != "" {
			b.WriteString(th.Summary + "\n\n")
		}
		for _, a := range th.Articles {
			fmt.Fprintf(&b, "- %s [%s](%s)", stars(a.Importance), a.Title, a.URL)
			if a.SourceName != "" {
				fmt.Fprintf(&b, " — %s", a.SourceName)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// digestCSS is inlined on purpose: the file has to render the same on a phone
// with no network as it does on the machine that wrote it, and an email client
// will not fetch a stylesheet either.
const digestCSS = `
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 24px 18px 48px;
    font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    color: #1c1c1e; background: #fff;
  }
  main { max-width: 40rem; margin: 0 auto; }
  h1 { font-size: 1.5rem; margin: 0 0 4px; }
  .date { color: #6b6b70; font-size: .85rem; margin-bottom: 24px; }
  .overview { font-size: 1.05rem; margin-bottom: 32px; }
  h2 { font-size: 1.05rem; text-transform: lowercase; letter-spacing: .02em;
       margin: 32px 0 6px; padding-bottom: 6px; border-bottom: 1px solid #e5e5ea; }
  .theme-summary { color: #3a3a3c; margin: 0 0 14px; }
  ul { list-style: none; margin: 0; padding: 0; }
  li { margin: 0 0 14px; }
  a { color: #0a58ca; text-decoration: none; }
  a:hover { text-decoration: underline; }
  .stars { color: #b8860b; font-size: .8rem; margin-right: 6px; white-space: nowrap; }
  .source { color: #6b6b70; font-size: .82rem; }
  footer { margin-top: 40px; color: #6b6b70; font-size: .78rem; }
  @media (prefers-color-scheme: dark) {
    body { color: #e8e8ea; background: #16161a; }
    h2 { border-bottom-color: #2c2c31; }
    .theme-summary { color: #b6b6bb; }
    a { color: #7aa7ff; }
    .date, .source, footer { color: #96969c; }
  }
`

// HTML renders a self-contained page: no stylesheet, no script, no image, so
// it opens anywhere and asks the network for nothing.
func HTML(d *Digest) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>AI digest · %s</title>\n", html.EscapeString(d.Date))
	b.WriteString("<style>" + digestCSS + "</style>\n</head>\n<body>\n<main>\n")

	b.WriteString("<h1>AI digest</h1>\n")
	fmt.Fprintf(&b, "<div class=\"date\">%s</div>\n", html.EscapeString(d.Date))
	if d.Overview != "" {
		fmt.Fprintf(&b, "<p class=\"overview\">%s</p>\n", html.EscapeString(d.Overview))
	}

	if len(d.Watchlist) > 0 {
		b.WriteString("<h2>watchlist</h2>\n<ul>\n")
		for _, hit := range d.Watchlist {
			b.WriteString("<li>")
			if hit.Article.URL != "" {
				fmt.Fprintf(&b, "<a href=\"%s\">%s</a>",
					html.EscapeString(hit.Article.URL), html.EscapeString(hit.Article.Title))
			} else {
				b.WriteString(html.EscapeString(hit.Article.Title))
			}
			fmt.Fprintf(&b, "<div class=\"source\">%s</div></li>\n", html.EscapeString(hit.Rule))
		}
		b.WriteString("</ul>\n")
	}
	for _, th := range d.Themes {
		fmt.Fprintf(&b, "<h2>%s</h2>\n", html.EscapeString(th.Name))
		if th.Summary != "" {
			fmt.Fprintf(&b, "<p class=\"theme-summary\">%s</p>\n", html.EscapeString(th.Summary))
		}
		b.WriteString("<ul>\n")
		for _, a := range th.Articles {
			b.WriteString("<li>")
			fmt.Fprintf(&b, "<span class=\"stars\">%s</span>", stars(a.Importance))
			if a.URL != "" {
				fmt.Fprintf(&b, "<a href=\"%s\">%s</a>", html.EscapeString(a.URL), html.EscapeString(a.Title))
			} else {
				b.WriteString(html.EscapeString(a.Title))
			}
			if a.SourceName != "" {
				fmt.Fprintf(&b, "<div class=\"source\">%s</div>", html.EscapeString(a.SourceName))
			}
			b.WriteString("</li>\n")
		}
		b.WriteString("</ul>\n")
	}

	b.WriteString("<footer>Written by giznews from your own feed.</footer>\n")
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

// stars is the importance, in the shape the rest of the app uses.
func stars(importance int) string {
	if importance < 0 {
		importance = 0
	}
	if importance > 3 {
		importance = 3
	}
	return strings.Repeat("★", importance) + strings.Repeat("☆", 3-importance)
}
