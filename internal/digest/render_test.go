package digest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/db"
)

func sampleDigest() *Digest {
	return &Digest{
		Date:     "2026-08-22",
		Overview: "A quiet week, mostly about long context.",
		Themes: []*Theme{
			{Name: "models", Summary: "Two releases.", Articles: []*db.Article{
				{Title: "OpenAI launches GPT-5", URL: "https://x/1?a=1&b=2", SourceName: "The Verge", Importance: 3},
				{Title: `Ten <script>alert(1)</script> "quotes" & ampersands`, URL: "https://x/2", SourceName: "HN", Importance: 1},
			}},
		},
	}
}

// A digest is read on a phone, in a mail client, on a machine with no network.
// So the page carries everything it needs and asks for nothing.
func TestHTMLIsSelfContainedAndEscaped(t *testing.T) {
	page := HTML(sampleDigest())

	for _, forbidden := range []string{"src=", "<link", "@import", "<script"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page reaches outside itself: %q", forbidden)
		}
	}
	if !strings.Contains(page, "<style>") {
		t.Error("the styles are not inlined")
	}
	if !strings.Contains(page, "width=device-width") {
		t.Error("no viewport: it would render at desktop width on a phone")
	}
	// A title is text, never markup.
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("a title was rendered as markup")
	}
	if !strings.Contains(page, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("the title was not escaped:\n%s", page)
	}
	if !strings.Contains(page, "https://x/1?a=1&amp;b=2") {
		t.Error("the URL was not escaped in the attribute")
	}
}

// Exported twice is the same bytes twice — otherwise a re-export cannot be
// told from a change.
func TestRenderingIsDeterministic(t *testing.T) {
	d := sampleDigest()
	if HTML(d) != HTML(d) {
		t.Fatal("two renders of one digest differ")
	}
	if Markdown(d) != Markdown(d) {
		t.Fatal("two markdown renders of one digest differ")
	}
	md := Markdown(d)
	if !strings.Contains(md, "★★★ [OpenAI launches GPT-5](https://x/1?a=1&b=2)") {
		t.Fatalf("markdown link:\n%s", md)
	}
}

// The desktop app has been storing digests under its own key names since
// before this existed; a digest exported today may well be one of those.
func TestLoadReadsBothStoredShapes(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	if err := Save(ctx, d, sampleDigest()); err != nil {
		t.Fatal(err)
	}
	mine, err := Load(ctx, d, "2026-08-22")
	if err != nil {
		t.Fatal(err)
	}
	if len(mine.Themes) != 1 || mine.Themes[0].Name != "models" || len(mine.Themes[0].Articles) != 2 {
		t.Fatalf("round trip lost something: %+v", mine.Themes)
	}

	// The shape the desktop app writes: "theme" instead of "name".
	if err := db.NewDigestRepo(d).Save(ctx, "2026-08-21", "From the app.",
		`[{"theme":"regulation","summary":"One ruling.","articles":[{"id":1,"title":"A ruling","url":"https://x/9"}]}]`); err != nil {
		t.Fatal(err)
	}
	theirs, err := Load(ctx, d, "2026-08-21")
	if err != nil {
		t.Fatal(err)
	}
	if len(theirs.Themes) != 1 || theirs.Themes[0].Name != "regulation" {
		t.Fatalf("a digest the app wrote came back nameless: %+v", theirs.Themes)
	}
	if len(theirs.Themes[0].Articles) != 1 || theirs.Themes[0].Articles[0].Title != "A ruling" {
		t.Fatalf("articles lost: %+v", theirs.Themes[0].Articles)
	}

	// A day nobody wrote a digest for must fail, not come back empty: the
	// caller is about to write a file.
	if _, err := Load(ctx, d, "2020-01-01"); err == nil {
		t.Fatal("expected a missing digest to be an error")
	}
}
