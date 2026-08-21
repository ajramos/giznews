package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/sources"
	"github.com/go-shiori/go-readability"
)

const manualSourceName = "Manual"

// ensureManualSource returns (creating if needed) the shared source used for
// articles ingested directly by URL. It is hidden from the sources picker.
func (a *App) ensureManualSource(ctx context.Context) (*db.Source, error) {
	repo := db.NewSourceRepo(a.db)
	s, err := repo.GetByName(ctx, manualSourceName)
	if err == nil {
		return s, nil
	}
	if err != db.ErrNotFound {
		return nil, err
	}
	s, err = repo.Create(ctx, db.NewSource{
		Name: manualSourceName, Type: db.SourceManual, Group: "manual", Enabled: false,
	})
	if err != nil {
		return nil, err
	}
	if err := repo.SetHidden(ctx, s.ID, true); err != nil {
		return nil, err
	}
	return s, nil
}

// IngestURL fetches a single article by URL, extracts its readable body, and
// upserts it as an article under the shared Manual source (deduped by URL).
func (a *App) IngestURL(ctx context.Context, raw string) (*ArticleDTO, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty url")
	}

	var out *ArticleDTO
	err := a.trackJob(ctx, "Ingest URL", "ingest", func(jctx context.Context, p *jobProgress) error {
		repo := db.NewArticleRepo(a.db)

		// Already imported (from a feed or a previous :url): don't stack a
		// second copy under a different GUID. Return the existing row.
		if existing, err := repo.FindByURL(jctx, raw); err == nil {
			p.Progress("done", 1, 1)
			out = toArticleDTO(existing)
			return nil
		} else if !errors.Is(err, db.ErrNotFound) {
			return err
		}

		src, err := a.ensureManualSource(jctx)
		if err != nil {
			return err
		}

		p.Progress("fetch", 0, 1)
		parsed, err := readability.FromURL(raw, 20*time.Second)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", raw, err)
		}
		md := sources.HTMLToMarkdown(parsed.Content)
		title := strings.TrimSpace(parsed.Title)
		if title == "" {
			title = raw
		}

		id, _, err := repo.Upsert(jctx, db.NewArticle{
			SourceID:    src.ID,
			GUID:        sha256Hex(raw),
			URL:         raw,
			Title:       title,
			Author:      strings.TrimSpace(parsed.Byline),
			ContentHTML: parsed.Content,
			ContentMD:   md,
			Status:      db.StatusUnread,
		})
		if err != nil {
			return err
		}
		p.Progress("done", 1, 1)

		art, err := repo.Get(jctx, id)
		if err != nil {
			return err
		}
		out = toArticleDTO(art)
		p.Message(title)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
