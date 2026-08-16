package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/kb"
)

// kbService builds the knowledge-graph service from config.
func (a *App) kbService() (*kb.Service, error) {
	prov, err := a.provider()
	if err != nil {
		return nil, err
	}
	return kb.NewService(a.db, a.cfg.ResolveVaultPath(), kb.Options{
		ImportanceThreshold: a.cfg.Classify.ImportanceThreshold,
		Model:               a.cfg.LLM.Model,
		UseLLM:              a.cfg.LLM.Enabled && prov != nil,
	}, prov, a.logger())
}

// KBResult is the desktop DTO for a kb build run.
type KBResult struct {
	AtomsCreated     int `json:"atoms_created"`
	ElectronsCreated int `json:"electrons_created"`
	ElectronsUpdated int `json:"electrons_updated"`
	MoleculesCreated int `json:"molecules_created"`
	ArticlesSkipped  int `json:"articles_skipped"`
}

// KBuild ingests pending articles into the knowledge graph.
func (a *App) KBuild(ctx context.Context) (*KBResult, error) {
	var result *KBResult
	err := a.trackJob(ctx, "Build knowledge graph", "kb", func(jctx context.Context, p *jobProgress) error {
		svc, err := a.kbService()
		if err != nil {
			return err
		}
		res, err := svc.Build(jctx)
		if err != nil {
			return err
		}
		result = &KBResult{
			AtomsCreated:     res.AtomsCreated,
			ElectronsCreated: res.ElectronsCreated,
			ElectronsUpdated: res.ElectronsUpdated,
			MoleculesCreated: res.MoleculesCreated,
			ArticlesSkipped:  res.ArticlesSkipped,
		}
		p.Message(fmt.Sprintf("%d atoms · %d electrons", res.AtomsCreated, res.ElectronsCreated))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// KSynthesize creates a molecule summarizing a category.
func (a *App) KSynthesize(ctx context.Context, category string) (*KBResult, error) {
	var result *KBResult
	err := a.trackJob(ctx, fmt.Sprintf("Synthesize %s", category), "kb", func(jctx context.Context, p *jobProgress) error {
		svc, err := a.kbService()
		if err != nil {
			return err
		}
		res, err := svc.Synthesize(jctx, category)
		if err != nil {
			return err
		}
		result = &KBResult{MoleculesCreated: res.MoleculesCreated}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// EnsureArticleNote creates (if missing) a knowledge-graph note for a single
// article, returning it so the graph panel can show it immediately.
func (a *App) EnsureArticleNote(ctx context.Context, articleID int64) (*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	note, err := svc.EnsureArticleNote(ctx, articleID)
	if err != nil {
		return nil, err
	}
	return toNoteDTO(note), nil
}

// GetArticleNote returns the Atom note created from an article (via the ingest
// mapping), or nil when the article has no note yet.
func (a *App) GetArticleNote(ctx context.Context, articleID int64) (*NoteDTO, error) {
	var noteID int64
	err := a.db.SQL().QueryRowContext(ctx,
		"SELECT COALESCE(note_id, 0) FROM ingests WHERE ref_type = 'article' AND ref_id = ?",
		fmt.Sprintf("%d", articleID)).Scan(&noteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if noteID == 0 {
		return nil, nil
	}
	note, err := db.NewKBRepo(a.db).Get(ctx, noteID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toNoteDTO(note), nil
}

func (a *App) ListNotes(ctx context.Context, noteType string) ([]*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	notes, err := svc.ListNotes(ctx, noteType, 200)
	if err != nil {
		return nil, err
	}
	out := make([]*NoteDTO, 0, len(notes))
	for _, n := range notes {
		out = append(out, toNoteDTO(n))
	}
	return out, nil
}

func (a *App) GetNote(ctx context.Context, id int64) (*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	n, err := svc.GetNote(ctx, id)
	if err != nil {
		return nil, err
	}
	return toNoteDTO(n), nil
}

func (a *App) GraphNeighbors(ctx context.Context, id int64) ([]*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	notes, err := svc.GraphNeighbors(ctx, id, 50)
	if err != nil {
		return nil, err
	}
	out := make([]*NoteDTO, 0, len(notes))
	for _, n := range notes {
		out = append(out, toNoteDTO(n))
	}
	return out, nil
}

func toNoteDTO(n *db.KBNote) *NoteDTO {
	dto := &NoteDTO{
		ID: n.ID, Type: string(n.Type), Title: n.Title, Slug: n.Slug,
		Content: n.Content, Tags: n.Tags, Wikilinks: n.Wikilinks, CreatedAt: n.CreatedAt,
	}
	// Frontmatter is a JSON map ({"category","source","url","rating","tags"})
	// for atoms; parse it so the reader can render rich metadata chips.
	var fm struct {
		Category string `json:"category"`
		Source   string `json:"source"`
		URL      string `json:"url"`
		Rating   int    `json:"rating"`
	}
	if n.Frontmatter != "" {
		_ = json.Unmarshal([]byte(n.Frontmatter), &fm)
		dto.Category = fm.Category
		dto.Source = fm.Source
		dto.URL = fm.URL
		dto.Rating = fm.Rating
	}
	return dto
}
